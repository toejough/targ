// Package runner provides the core implementation for the targ CLI tool.
package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/toejough/targ/internal/discover"
	"github.com/toejough/targ/internal/flags"
	"github.com/toejough/targ/internal/help"
)

// ContentPatch represents a string replacement to apply to file content.
type ContentPatch struct {
	Old string
	New string
}

// CreateOptions holds options for the --create command.
type CreateOptions struct {
	Path     []string // Group path components (e.g., ["dev", "lint"] for "dev lint fast")
	Name     string   // Target name (e.g., "fast")
	ShellCmd string   // Shell command to execute
	Deps     []string // Dependency target names
	Cache    []string // Cache patterns
	Watch    []string // Watch patterns
	Timeout  string   // Duration string (e.g. "30s")
	Times    int      // Run count (0 = unset)
	Retry    bool     // Continue on failure
	Backoff  string   // "duration,multiplier" (e.g. "1s,2.0")
	DepMode  string   // "serial" or "parallel"
}

// FileOps abstracts file system operations for testing.
type FileOps interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	ReadDir(name string) ([]fs.DirEntry, error)
	MkdirAll(path string, perm fs.FileMode) error
	Stat(name string) (fs.FileInfo, error)
}

// OSFileOps implements FileOps using the real filesystem.
type OSFileOps struct{}

// MkdirAll creates a directory and all parents.
func (OSFileOps) MkdirAll(path string, perm fs.FileMode) error {
	err := os.MkdirAll(path, perm)
	if err != nil {
		return fmt.Errorf("creating directory %s: %w", path, err)
	}

	return nil
}

// ReadDir reads a directory.
func (OSFileOps) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", name, err)
	}

	return entries, nil
}

// ReadFile reads a file from the filesystem.
func (OSFileOps) ReadFile(name string) ([]byte, error) {
	//nolint:gosec // build tool reads user source files by design
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", name, err)
	}

	return data, nil
}

// Stat returns file info.
func (OSFileOps) Stat(name string) (fs.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err //nolint:wrapcheck // Callers need unwrapped error for os.IsNotExist checks
	}

	return info, nil
}

// WriteFile writes data to a file.
func (OSFileOps) WriteFile(name string, data []byte, perm fs.FileMode) error {
	err := os.WriteFile(name, data, perm)
	if err != nil {
		return fmt.Errorf("writing file %s: %w", name, err)
	}

	return nil
}

// SyncOptions holds options for the --sync command.
type SyncOptions struct {
	PackagePath string // Module path to sync (e.g., "github.com/foo/bar")
}

// TargDependency represents a targ module dependency for isolated builds.
type TargDependency struct {
	ModulePath string
	Version    string
	ReplaceDir string
}

// TargFlags holds extracted targ-specific flags.
type TargFlags struct {
	NoBinaryCache bool   // Disable binary caching
	SourceDir     string // Source directory for targ files
}

// AddImportToTargFile adds a blank import for the given package to the targ file.
func AddImportToTargFile(path, packagePath string) error {
	return AddImportToTargFileWithFileOps(OSFileOps{}, path, packagePath)
}

// AddImportToTargFileWithFileOps adds a blank import and a DeregisterFrom call
// using injected file operations. The DeregisterFrom call is added to init()
// so targets from the synced package are deregistered by default, preventing conflicts.
func AddImportToTargFileWithFileOps(fileOps FileOps, path, packagePath string) error {
	content, err := fileOps.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	// Add the blank import
	addBlankImport(file, packagePath)

	// Ensure "github.com/toejough/targ" import exists (needed for DeregisterFrom)
	ensureTargImport(file)

	// Add DeregisterFrom call to init()
	addDeregisterFromToInit(file, packagePath)

	// Format and write back
	var buf bytes.Buffer

	err = format.Node(&buf, fset, file)
	if err != nil {
		return fmt.Errorf("formatting file: %w", err)
	}

	err = fileOps.WriteFile(path, buf.Bytes(), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// AddTargetToFileWithFileOps adds a target variable to an existing targ file using injected file ops.
func AddTargetToFileWithFileOps(fileOps FileOps, path string, opts CreateOptions) error {
	content, err := fileOps.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	fullPath := append(opts.Path, opts.Name) //nolint:gocritic // intentional copy
	varName := PathToPascal(fullPath)

	// Check for duplicates - either var declaration (old pattern) or .Name("targetname") (new inline pattern)
	contentStr := string(content)
	if strings.Contains(contentStr, fmt.Sprintf("var %s = ", varName)) ||
		strings.Contains(contentStr, fmt.Sprintf(`.Name("%s")`, opts.Name)) {
		return fmt.Errorf("%w: %s", errDuplicateTarget, strings.Join(fullPath, "/"))
	}

	result, err := buildTargetCode(varName, opts)
	if err != nil {
		return err
	}

	groupMods := generateGroupModifications(opts.Path, varName, string(content))

	modifiedContent := string(content)
	for _, patch := range groupMods.ContentPatches {
		modifiedContent = strings.Replace(modifiedContent, patch.Old, patch.New, 1)
	}

	// Build inline targ expression for Register() instead of using var
	targExpr, err := buildTargetExpression(opts)
	if err != nil {
		return err
	}

	// Check if we need time import based on opts
	needsTimeImport := opts.Timeout != "" || opts.Backoff != ""
	if needsTimeImport && !strings.Contains(modifiedContent, `"time"`) {
		modifiedContent = addTimeImport(modifiedContent)
	}

	registerArg := targExpr
	if len(opts.Path) > 0 {
		// For nested groups, we still register the top-level group var
		registerArg = PathToPascal(opts.Path[:1])
		// But we need to keep the var declaration for group members
		modifiedContent = modifiedContent + result.code
	}

	newContent := modifiedContent + groupMods.newCode

	newContent, err = addRegisterArgToInit(newContent, registerArg)
	if err != nil {
		return err
	}

	err = fileOps.WriteFile(path, []byte(newContent), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// AddTargetToFile adds a target variable to an existing targ file.

// AddTargetToFileWithOptions adds a target with full options to an existing targ file.
func AddTargetToFileWithOptions(path string, opts CreateOptions) error {
	return AddTargetToFileWithFileOps(OSFileOps{}, path, opts)
}

// CheckImportExists checks if a blank import for the given package already exists in the file.
func CheckImportExists(path, packagePath string) (bool, error) {
	return CheckImportExistsWithFileOps(OSFileOps{}, path, packagePath)
}

// CheckImportExistsWithFileOps checks for an import using injected file operations.
func CheckImportExistsWithFileOps(fileOps FileOps, path, packagePath string) (bool, error) {
	content, err := fileOps.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading file: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, content, parser.ImportsOnly)
	if err != nil {
		return false, fmt.Errorf("parsing file: %w", err)
	}

	for _, imp := range file.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}

		if importPath == packagePath {
			return true, nil
		}
	}

	return false, nil
}

// ContainsHelpFlag returns true if args contain --help or -h.
func ContainsHelpFlag(args []string) bool {
	for _, a := range args {
		if a == helpLong || a == helpShort {
			return true
		}
	}

	return false
}

// ConvertFuncTargetToString converts a function target to a string target.
// Returns true if conversion was performed, false if target not found or not convertible.
func ConvertFuncTargetToString(filePath, targetName string) (bool, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parsing file: %w", err)
	}

	info := findFuncTarget(file, targetName)
	if info == nil {
		return false, nil
	}

	// Find the function declaration and extract shell command
	shellCmd, funcDecl := extractShellCommand(file, info.funcIdent.Name)
	if shellCmd == "" {
		//nolint:err113 // specific error message for user feedback
		return false, fmt.Errorf("function %s is not a simple targ.Run call", info.funcIdent.Name)
	}

	// Replace function reference with string
	info.call.Args[0] = &ast.BasicLit{
		Kind:  token.STRING,
		Value: strconv.Quote(shellCmd),
	}

	removeFuncDecl(file, funcDecl)

	err = writeFormattedFile(filePath, fset, file)
	if err != nil {
		return false, fmt.Errorf("writing file: %w", err)
	}

	return true, nil
}

// ConvertStringTargetToFunc converts a string target to a function target.
// Returns true if conversion was performed, false if target not found or not a string target.
func ConvertStringTargetToFunc(filePath, targetName string) (bool, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parsing file: %w", err)
	}

	info := findStringTarget(file, targetName)
	if info == nil {
		return false, nil
	}

	funcName := targetNameToFuncName(targetName)

	// Replace string argument with function reference
	info.call.Args[0] = ast.NewIdent(funcName)

	// Generate and add function declaration
	file.Decls = append(file.Decls, generateShellFunc(funcName, info.shellCmd))

	err = writeFormattedFile(filePath, fset, file)
	if err != nil {
		return false, fmt.Errorf("writing file: %w", err)
	}

	return true, nil
}

// CreateGroupMemberPatch creates a patch to add a new member to an existing group.
// Returns nil if the member already exists in the group.
func CreateGroupMemberPatch(content, groupVarName, newMember string) *ContentPatch {
	// Find the group declaration: var GroupName = targ.Group("name", member1, member2)
	// We need to add newMember before the closing parenthesis

	// Find the start of the group declaration
	pattern := fmt.Sprintf("var %s = targ.Group(", groupVarName)

	startIdx := strings.Index(content, pattern)
	if startIdx == -1 {
		return nil
	}

	// Find the closing parenthesis for this declaration
	// We need to handle nested parentheses (though unlikely in this context)
	parenStart := startIdx + len(pattern)
	parenCount := 1
	endIdx := -1

	for i := parenStart; i < len(content) && parenCount > 0; i++ {
		switch content[i] {
		case '(':
			parenCount++
		case ')':
			parenCount--
			if parenCount == 0 {
				endIdx = i
			}
		}
	}

	if endIdx == -1 {
		return nil
	}

	// Extract the current group declaration
	oldDecl := content[startIdx : endIdx+1]

	// Check if member already exists
	if strings.Contains(oldDecl, newMember) {
		return nil
	}

	// Create the new declaration by inserting the member before the closing paren
	newDecl := content[startIdx:endIdx] + ", " + newMember + ")"

	return &ContentPatch{
		Old: oldDecl,
		New: newDecl,
	}
}

// EnsureFallbackModuleRoot creates a fallback module root for isolated builds.
func EnsureFallbackModuleRoot(startDir, modulePath string, dep TargDependency) (string, error) {
	hash := sha256.Sum256([]byte(startDir))

	root := filepath.Join(projectCacheDir(startDir), "mod", hex.EncodeToString(hash[:8]))

	//nolint:gosec,mnd // standard cache directory permissions
	err := os.MkdirAll(root, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating fallback module directory: %w", err)
	}

	err = linkModuleRoot(startDir, root)
	if err != nil {
		return "", err
	}

	err = writeFallbackGoMod(root, modulePath, dep)
	if err != nil {
		return "", err
	}

	err = touchFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return "", err
	}

	return root, nil
}

// ExtractTargFlags extracts targ-specific flags from args.
// Returns the flags and remaining args to pass to the binary.
//
// Position-sensitive flags (like --source, -s) are only recognized when they
// appear BEFORE the first target name. After a target is seen, these flags
// are passed through to the binary.
func ExtractTargFlags(args []string) (flags TargFlags, remaining []string) {
	remaining = make([]string, 0, len(args))
	seenTarget := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		isFlag := strings.HasPrefix(arg, "-")

		// Track when we see a target name (first non-flag)
		if !isFlag && !seenTarget {
			seenTarget = true
		}

		switch arg {
		case "--no-binary-cache":
			flags.NoBinaryCache = true
		case "--no-cache":
			// Deprecated: use --no-binary-cache instead
			fmt.Fprintln(
				os.Stderr,
				"warning: --no-cache is deprecated, use --no-binary-cache instead",
			)

			flags.NoBinaryCache = true
		case "--source", "-s":
			// Position-sensitive: only before first target
			if !seenTarget && i+1 < len(args) {
				i++
				flags.SourceDir = args[i]
			} else {
				remaining = append(remaining, arg)
			}
		default:
			remaining = append(remaining, arg)
		}
	}

	return flags, remaining
}

// FindModuleForPath walks up from the given path to find the nearest go.mod.
// Returns the module root directory, module path, whether found, and any error.
func FindModuleForPath(path string) (string, string, bool, error) {
	// Start from the directory containing the path
	dir := path

	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}

	for {
		modPath := filepath.Join(dir, "go.mod")

		//nolint:gosec // build tool reads go.mod files by design
		data, err := os.ReadFile(modPath)
		if err == nil {
			modulePath := parseModulePath(string(data))
			if modulePath == "" {
				return "", "", true, fmt.Errorf("%w: %s", errModulePathNotFound, modPath)
			}

			return dir, modulePath, true, nil
		}

		if !os.IsNotExist(err) {
			return "", "", false, fmt.Errorf("reading go.mod: %w", err)
		}

		// Move up to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}

		dir = parent
	}

	return "", "", false, nil
}

// FindOrCreateTargFile finds an existing targ file in the current directory or creates a new one.
func FindOrCreateTargFile(startDir string) (string, error) {
	return FindOrCreateTargFileWithFileOps(OSFileOps{}, startDir)
}

// FindOrCreateTargFileWithFileOps finds or creates a targ file using injected file operations.
func FindOrCreateTargFileWithFileOps(fileOps FileOps, startDir string) (string, error) {
	// Look for existing targ files in the current directory or descendants.
	path, found, err := findTargFileInTree(fileOps, startDir)
	if err != nil {
		return "", err
	}

	if found {
		return path, nil
	}

	// No existing targ file found, create a new one.
	targFile := filepath.Join(startDir, "targs.go")
	pkgName := filepath.Base(startDir)
	// Sanitize package name (remove invalid characters)
	pkgName = strings.ReplaceAll(pkgName, "-", "")

	pkgName = strings.ReplaceAll(pkgName, ".", "")
	if pkgName == "" {
		pkgName = defaultPackageName
	}

	content := fmt.Sprintf(`//go:build targ

package %s

import "github.com/toejough/targ"

func init() {
	targ.Register()
}
`, pkgName)

	err = fileOps.WriteFile(targFile, []byte(content), filePermissionsForCode)
	if err != nil {
		return "", fmt.Errorf("creating targ file: %w", err)
	}

	return targFile, nil
}

// HasTargBuildTag returns true if the file has the targ build tag.
func HasTargBuildTag(path string) bool {
	return HasTargBuildTagWithFileOps(OSFileOps{}, path)
}

// HasTargBuildTagWithFileOps checks for the targ build tag using injected file operations.
func HasTargBuildTagWithFileOps(fileOps FileOps, path string) bool {
	content, err := fileOps.ReadFile(path)
	if err != nil {
		return false
	}
	// Check for //go:build targ at the start
	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "//go:build") && strings.Contains(line, "targ") {
			return true
		}
		// Stop at package declaration
		if strings.HasPrefix(line, "package ") {
			break
		}
	}

	return false
}

// IsValidTargetName returns true if the name is valid for a target (kebab-case).
// Must start with lowercase letter, contain only lowercase letters, numbers, and hyphens,
// and cannot end with a hyphen.
func IsValidTargetName(name string) bool {
	return validTargetNameRe.MatchString(name)
}

// KebabToPascal converts kebab-case to PascalCase.
func KebabToPascal(s string) string {
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return strings.Join(parts, "")
}

// NamespacePaths computes collapsed namespace paths for a set of files.
func NamespacePaths(files []string, root string) (map[string][]string, error) {
	if len(files) == 0 {
		return map[string][]string{}, nil
	}

	raw := make(map[string][]string, len(files))

	paths := make([][]string, 0, len(files))
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return nil, fmt.Errorf("getting relative path for %s: %w", file, err)
		}

		rel = filepath.ToSlash(rel)

		parts := strings.Split(rel, "/")
		if len(parts) == 0 {
			parts = []string{filepath.Base(file)}
		}

		last := parts[len(parts)-1]
		parts[len(parts)-1] = strings.TrimSuffix(last, filepath.Ext(last))
		raw[file] = parts
		paths = append(paths, parts)
	}

	common := append([]string(nil), paths[0]...)
	for _, p := range paths[1:] {
		common = commonPrefix(common, p)
		if len(common) == 0 {
			break
		}
	}

	trimmed := make(map[string][]string, len(files))
	for file, parts := range raw {
		if len(common) >= len(parts) {
			trimmed[file] = nil
			continue
		}

		trimmed[file] = append([]string(nil), parts[len(common):]...)
	}

	return compressNamespacePaths(trimmed), nil
}

// ParseCreateArgs parses arguments for --create; the shell command is always the last argument.
func ParseCreateArgs(args []string) (CreateOptions, error) {
	var opts CreateOptions

	if len(args) < 2 { //nolint:mnd // minimum: name + command
		return opts, errCreateUsage
	}

	opts.ShellCmd = args[len(args)-1]

	parser := &createArgParser{opts: opts, remaining: args[:len(args)-1]}
	for parser.i < len(parser.remaining) {
		err := parser.parseArg()
		if err != nil {
			return opts, err
		}
	}

	if len(parser.pathAndName) < 1 {
		return opts, errCreateUsage
	}

	parser.opts.Name = parser.pathAndName[len(parser.pathAndName)-1]
	if len(parser.pathAndName) > 1 {
		parser.opts.Path = parser.pathAndName[:len(parser.pathAndName)-1]
	}

	return parser.opts, nil
}

// ParseHelpRequest parses args to determine if help was requested and if a target was specified.
func ParseHelpRequest(args []string) (bool, bool) {
	helpRequested := false
	sawTarget := false

	for _, arg := range args {
		if arg == "--" {
			break
		}

		if arg == helpLong || arg == helpShort {
			helpRequested = true
			continue
		}

		if strings.HasPrefix(arg, "-") {
			continue
		}

		sawTarget = true
	}

	return helpRequested, sawTarget
}

// ParseSyncArgs parses arguments after --sync into SyncOptions.
// Format: --sync <package-path>
func ParseSyncArgs(args []string) (SyncOptions, error) {
	var opts SyncOptions

	if len(args) < 1 {
		return opts, errSyncUsage
	}

	opts.PackagePath = args[0]

	// Validate that it looks like a module path
	if !looksLikeModulePath(opts.PackagePath) {
		return opts, fmt.Errorf("%w: %s", errInvalidPackagePath, opts.PackagePath)
	}

	return opts, nil
}

// PathToPascal converts a path like ["dev", "lint", "fast"] to "DevLintFast".
func PathToPascal(path []string) string {
	var result strings.Builder
	for _, p := range path {
		result.WriteString(KebabToPascal(p))
	}

	return result.String()
}

// PrintCreateHelp writes structured help for --create to w.
func PrintCreateHelp(w io.Writer) {
	output := help.New("targ --create").
		WithDescription("Create a new target in the nearest targ file.").
		WithUsage(`targ --create [group...] <name> [flags...] "<shell-command>"`).
		AddPositionals(
			help.Positional{Name: "group", Placeholder: "<group...>"},
			help.Positional{Name: "name", Placeholder: "<name>", Required: true},
			help.Positional{
				Name:        "shell-command",
				Placeholder: `"<shell-command>"`,
				Required:    true,
			},
		).
		AddCommandFlags(
			help.Flag{Long: "--deps", Placeholder: "<targets...>", Desc: "Deps to run first"},
			help.Flag{Long: "--cache", Placeholder: "<patterns...>", Desc: "Skip if unchanged"},
			help.Flag{Long: "--watch", Placeholder: "<patterns...>", Desc: "Re-run on change"},
			help.Flag{Long: "--timeout", Placeholder: "<duration>", Desc: "Execution timeout"},
			help.Flag{Long: "--times", Placeholder: "<n>", Desc: "Run n times"},
			help.Flag{Long: "--retry", Desc: "Continue on failure"},
			help.Flag{Long: "--backoff", Placeholder: "<duration,mult>", Desc: "Backoff (1s,2.0)"},
			help.Flag{Long: "--dep-mode", Placeholder: "<mode>", Desc: "serial or parallel"},
		).
		AddFormats(
			help.Format{
				Name: "duration",
				Desc: "<int><unit> where unit is s (seconds), m (minutes), h (hours)",
			},
		).
		AddExamples(
			help.Example{Code: `targ --create test "go test ./..."`},
			help.Example{
				Code: `targ --create dev lint --deps fmt --cache "**/*.go" "golangci-lint run"`,
			},
			help.Example{Code: `targ --create test --timeout 30s --retry "go test ./..."`},
		).
		Render()
	_, _ = fmt.Fprint(w, output)
}

// PrintSyncHelp writes structured help for --sync to w.
func PrintSyncHelp(w io.Writer) {
	output := help.New("targ --sync").
		WithDescription("Sync targets from a remote package.").
		WithUsage("targ --sync <package-path>").
		AddExamples(
			help.Example{Code: "targ --sync github.com/user/repo"},
			help.Example{Code: "targ --sync github.com/user/repo/tools"},
		).
		Render()
	_, _ = fmt.Fprint(w, output)
}

// PrintToFuncHelp writes structured help for --to-func to w.
func PrintToFuncHelp(w io.Writer) {
	output := help.New("targ --to-func").
		WithDescription("Convert a string target to a function target.").
		WithUsage("targ --to-func <target-name>").
		AddExamples(
			help.Example{Code: "targ --to-func test"},
			help.Example{Code: "targ --to-func dev/lint"},
		).
		Render()
	_, _ = fmt.Fprint(w, output)
}

// PrintToStringHelp writes structured help for --to-string to w.
func PrintToStringHelp(w io.Writer) {
	output := help.New("targ --to-string").
		WithDescription("Convert a function target to a string target.").
		WithUsage("targ --to-string <target-name>").
		AddExamples(
			help.Example{Code: "targ --to-string test"},
			help.Example{Code: "targ --to-string dev/lint"},
		).
		Render()
	_, _ = fmt.Fprint(w, output)
}

// Run executes the targ CLI with the given binary name and arguments.
// Returns the exit code to pass to os.Exit().
func Run() int {
	// Guard against nil os.Args (should never happen, but satisfies static analysis)
	if len(os.Args) == 0 {
		fmt.Fprintln(os.Stderr, "error: os.Args is empty")
		return 1
	}

	r := &targRunner{
		binArg: os.Args[0],
		args:   os.Args[1:],
		errOut: os.Stderr,
	}

	return r.run()
}

// WriteBootstrapFile creates a temporary bootstrap file and returns its path and cleanup function.
func WriteBootstrapFile(dir string, data []byte) (string, func() error, error) {
	//nolint:gosec,mnd // standard directory permissions for bootstrap
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", nil, fmt.Errorf("creating bootstrap directory: %w", err)
	}

	temp, err := os.CreateTemp(dir, "targ_bootstrap_*.go")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}

	tempFile := temp.Name()

	_, err = temp.Write(data)
	if err != nil {
		_ = temp.Close()
		return "", nil, fmt.Errorf("writing bootstrap file: %w", err)
	}

	err = temp.Close()
	if err != nil {
		return "", nil, fmt.Errorf("closing bootstrap file: %w", err)
	}

	cleanup := func() error {
		err := os.Remove(tempFile)

		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing bootstrap file: %w", err)
		}

		return nil
	}

	return tempFile, cleanup, nil
}

// unexported constants.
const (
	bootstrapTemplate = `
package main

import (
	"github.com/toejough/targ"
{{- range .BlankImports }}
	_ "{{ . }}"
{{- end }}
)

func main() {
	targ.EnableCleanup()
	targ.ExecuteRegisteredWithOptions(targ.RunOptions{
		Description: {{ printf "%q" .Description }},
	})
}
`
	buildTag               = "targ" // build tag for targ files
	commandNamePadding     = 2      // Padding after command name column
	completeCommand        = "__complete"
	defaultPackageName     = "main" // default package name for created targ files
	defaultTargModulePath  = "github.com/toejough/targ"
	filePermissionsForCode = 0o644 // standard file permissions for created source files
	helpIndentWidth        = 4     // Leading spaces in help output
	helpLong               = "--help"
	helpShort              = "-h"
	isolatedModuleName     = "targ.build.local"
	minArgsForCompletion   = 2      // Minimum args for __complete (binary + arg)
	minCommandNameWidth    = 10     // Minimum column width for command names in help output
	pkgNameMain            = "main" // package main check for targ files
	targLocalModule        = "targ.local"
)

// unexported variables.
var (
	errBackoffFormat = errors.New("backoff must be duration,multiplier (e.g. 1s,2.0)")
	errCreateUsage   = errors.New(
		"usage: targ --create [group...] <name> [--deps ...] [--cache/--watch ...]" +
			" [--timeout/--times/--retry/--backoff/--dep-mode] \"<shell-command>\"",
	)
	errDuplicateTarget    = errors.New("target already exists")
	errInvalidDependency  = errors.New("invalid dependency name")
	errInvalidGroup       = errors.New("invalid group name")
	errInvalidPackagePath = errors.New(
		"invalid package path: must be a module path (e.g., github.com/user/repo)",
	)
	errInvalidTarget          = errors.New("invalid target name")
	errInvalidUTF8Path        = errors.New("invalid utf-8 path in tagged file")
	errModulePathNotFound     = errors.New("module path not found")
	errNoExplicitRegistration = errors.New(
		"package does not use explicit registration (targ.Register in init)",
	)
	errPackageAlreadySynced  = errors.New("package already synced")
	errPackageMainNotAllowed = errors.New("targ files cannot use 'package main'")
	errSyncUsage             = errors.New("usage: targ --sync <package-path>")
	errUnknownCommand        = errors.New("unknown command")
	errUnknownCreateFlag     = errors.New("unknown flag")
	validTargetNameRe        = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)
)

type bootstrapBuilder struct {
	moduleRoot          string
	modulePath          string
	explicitRegPackages []string // import paths for packages using targ.Register()
}

func (b *bootstrapBuilder) buildResult() bootstrapData {
	return bootstrapData{
		BlankImports: b.explicitRegPackages,
		Description:  "Targ discovers and runs build targets you write in Go.",
	}
}

func (b *bootstrapBuilder) computeImportPath(dir string) string {
	rel, err := filepath.Rel(b.moduleRoot, dir)
	if err != nil || rel == "." {
		return b.modulePath
	}

	return b.modulePath + "/" + filepath.ToSlash(rel)
}

func (b *bootstrapBuilder) processPackage(info discover.PackageInfo) error {
	if !info.UsesExplicitRegistration {
		return fmt.Errorf("%w: %s", errNoExplicitRegistration, info.Package)
	}

	importPath := b.computeImportPath(info.Dir)
	b.explicitRegPackages = append(b.explicitRegPackages, importPath)

	return nil
}

type bootstrapData struct {
	Description  string
	BlankImports []string // import paths for explicit registration packages
}

type buildContext struct {
	usingFallback bool
	buildRoot     string
	importRoot    string
}

// printMultiModuleHelp prints aggregated help for all modules.
type cmdEntry struct {
	name        string
	description string
}

type commandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// createArgParser holds state for parsing create arguments.
type createArgParser struct {
	opts        CreateOptions
	pathAndName []string
	remaining   []string
	i           int
}

func (p *createArgParser) parseArg() error {
	arg := p.remaining[p.i]

	switch arg {
	case "--deps":
		p.parseListFlag(&p.opts.Deps)
	case "--cache":
		p.parseListFlag(&p.opts.Cache)
	case "--watch":
		p.parseListFlag(&p.opts.Watch)
	case "--timeout":
		return p.parseSingleFlag("--timeout", &p.opts.Timeout)
	case "--times":
		return p.parseTimesFlag()
	case "--retry":
		p.opts.Retry = true
		p.i++
	case "--backoff":
		return p.parseSingleFlag("--backoff", &p.opts.Backoff)
	case "--dep-mode":
		return p.parseDepModeFlag()
	default:
		return p.parsePositionalOrUnknown(arg)
	}

	return nil
}

func (p *createArgParser) parseDepModeFlag() error {
	p.i++

	mode, newI, err := parseSingleValueArg(p.remaining, p.i, "--dep-mode")
	if err != nil {
		return err
	}

	if mode != "serial" && mode != "parallel" {
		return fmt.Errorf("%w: --dep-mode must be serial or parallel", errCreateUsage)
	}

	p.opts.DepMode = mode
	p.i = newI

	return nil
}

func (p *createArgParser) parseListFlag(target *[]string) {
	p.i++
	values, newI := collectListArg(p.remaining, p.i)
	*target = values
	p.i = newI
}

func (p *createArgParser) parsePositionalOrUnknown(arg string) error {
	if strings.HasPrefix(arg, "--") {
		return fmt.Errorf("%w: %s", errUnknownCreateFlag, arg)
	}

	p.pathAndName = append(p.pathAndName, arg)
	p.i++

	return nil
}

func (p *createArgParser) parseSingleFlag(flagName string, target *string) error {
	p.i++

	val, newI, err := parseSingleValueArg(p.remaining, p.i, flagName)
	if err != nil {
		return err
	}

	*target = val
	p.i = newI

	return nil
}

func (p *createArgParser) parseTimesFlag() error {
	p.i++

	val, newI, err := parseSingleValueArg(p.remaining, p.i, "--times")
	if err != nil {
		return err
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("%w: --times value must be an integer", errCreateUsage)
	}

	p.opts.Times = n
	p.i = newI

	return nil
}

// funcTargetInfo holds info about a function-based target found during search.
type funcTargetInfo struct {
	call      *ast.CallExpr
	funcIdent *ast.Ident
}

type groupModifications struct {
	newCode        string         // New group declarations to append
	ContentPatches []ContentPatch // Modifications to existing content
}

type listOutput struct {
	Commands []commandInfo `json:"commands"`
}

type moduleBootstrap struct {
	code     []byte
	cacheKey string
}

type moduleRegistry struct {
	BinaryPath string
	ModuleRoot string
	ModulePath string
	Commands   []commandInfo
}

type moduleTargets struct {
	ModuleRoot string
	ModulePath string
	Packages   []discover.PackageInfo
}

type namespaceNode struct {
	Name     string
	File     string
	Children map[string]*namespaceNode
	TypeName string
	VarName  string
}

// canCompress returns true if this node should be compressed (skipped in output).
func (n *namespaceNode) canCompress() bool {
	return len(n.Children) == 1 && n.File == ""
}

// collectCompressedPaths walks the tree and collects compressed paths.
// Assumes Children is always non-nil (enforced by insertPath and constructors).
func (n *namespaceNode) collectCompressedPaths(
	out map[string][]string,
	prefix []string,
	isRoot bool,
) {
	// Skip single-child intermediate nodes (compression)
	if !isRoot && n.canCompress() {
		for _, child := range n.Children {
			child.collectCompressedPaths(out, prefix, false)
		}

		return
	}

	if !isRoot {
		prefix = append(prefix, n.Name)
	}

	if n.File != "" {
		out[n.File] = append([]string(nil), prefix...)
	}

	for _, child := range n.sortedChildren() {
		child.collectCompressedPaths(out, prefix, false)
	}
}

// insertPath inserts a file path into the namespace tree.
func (n *namespaceNode) insertPath(file string, parts []string) {
	node := n
	for _, part := range parts {
		child := node.Children[part]
		if child == nil {
			child = &namespaceNode{Name: part, Children: make(map[string]*namespaceNode)}
			node.Children[part] = child
		}

		node = child
	}

	node.File = file
}

// sortedChildren returns children in sorted name order.
func (n *namespaceNode) sortedChildren() []*namespaceNode {
	names := make([]string, 0, len(n.Children))
	for name := range n.Children {
		names = append(names, name)
	}

	sort.Strings(names)

	children := make([]*namespaceNode, 0, len(names))
	for _, name := range names {
		if child := n.Children[name]; child != nil {
			children = append(children, child)
		}
	}

	return children
}

type osFileSystem struct{}

func (osFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", name, err)
	}

	return entries, nil
}

//nolint:gosec // build tool reads user source files by design
func (osFileSystem) ReadFile(name string) ([]byte, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", name, err)
	}

	return data, nil
}

func (osFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	err := os.WriteFile(name, data, perm)
	if err != nil {
		return fmt.Errorf("writing file %s: %w", name, err)
	}

	return nil
}

// stringTargetInfo holds info about a string-based target found during search.
type stringTargetInfo struct {
	call     *ast.CallExpr
	shellCmd string
}

type targRunner struct {
	binArg        string
	args          []string
	errOut        io.Writer
	startDir      string
	noBinaryCache bool
}

func (r *targRunner) buildAndRun(
	importRoot, binaryPath, targBinName string,
	bootstrapCode []byte,
) int {
	return r.buildAndRunWithOptions(importRoot, binaryPath, targBinName, bootstrapCode, false)
}

func (r *targRunner) buildAndRunIsolated(
	isolatedDir, binaryPath, targBinName string,
	bootstrapCode []byte,
) int {
	return r.buildAndRunWithOptions(isolatedDir, binaryPath, targBinName, bootstrapCode, true)
}

func (r *targRunner) buildAndRunWithOptions(
	buildDir, binaryPath, targBinName string,
	bootstrapCode []byte,
	isolated bool,
) int {
	bootstrapDir := r.resolveBootstrapDir(buildDir, isolated)

	tempFile, cleanupTemp, err := WriteBootstrapFile(bootstrapDir, bootstrapCode)
	if err != nil {
		r.logError("Error writing bootstrap file", err)
		return r.exitWithCleanup(1)
	}

	defer func() { _ = cleanupTemp() }()

	err = r.executeBuild(buildDir, binaryPath, tempFile, isolated)
	if err != nil {
		r.logError("Error building command", err)
		return r.exitWithCleanup(1)
	}

	return r.executeBuiltBinary(binaryPath, targBinName)
}

func (r *targRunner) discoverPackages() ([]discover.PackageInfo, error) {
	infos, err := discover.Discover(osFileSystem{}, discover.Options{
		StartDir: r.startDir,
		BuildTag: buildTag,
	})
	if err != nil {
		return nil, fmt.Errorf("error discovering commands: %w", err)
	}

	// Validate no package main in targ files
	for _, info := range infos {
		if info.Package == pkgNameMain {
			return nil, fmt.Errorf(
				"%w (found in %s); use a named package instead, e.g., 'package targets' or 'package dev'",
				errPackageMainNotAllowed,
				info.Dir,
			)
		}
	}

	return infos, nil
}

func (r *targRunner) dispatchFlagCommand(flagLong string, remaining []string) (int, bool) {
	switch flagLong {
	case "create":
		return r.handleCreateFlag(remaining), true
	case "sync":
		return r.handleSyncFlag(remaining), true
	case "to-func":
		return r.handleToFuncFlag(remaining), true
	case "to-string":
		return r.handleToStringFlag(remaining), true
	}

	return 0, false
}

func (r *targRunner) executeBuild(buildDir, binaryPath, tempFile string, isolated bool) error {
	buildArgs := []string{"build", "-tags", buildTag, "-o", binaryPath}
	if isolated {
		buildArgs = append(buildArgs, "-mod=mod")
	}

	buildArgs = append(buildArgs, tempFile)

	//nolint:gosec // build tool runs go build by design
	buildCmd := exec.CommandContext(context.Background(), "go", buildArgs...)

	var buildOutput bytes.Buffer

	buildCmd.Stdout = io.Discard
	buildCmd.Stderr = &buildOutput
	buildCmd.Dir = buildDir

	err := buildCmd.Run()
	if err != nil {
		if buildOutput.Len() > 0 {
			_, _ = fmt.Fprint(r.errOut, buildOutput.String())
		}

		return fmt.Errorf("running go build: %w", err)
	}

	return nil
}

func (r *targRunner) executeBuiltBinary(binaryPath, targBinName string) int {
	//nolint:gosec // build tool runs compiled binary by design
	cmd := exec.CommandContext(context.Background(), binaryPath, r.args...)

	cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = r.errOut
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		return r.exitWithCleanup(1)
	}

	return 0
}

func (r *targRunner) exitWithCleanup(code int) int {
	return code
}

func (r *targRunner) handleCreateFlag(args []string) int {
	if ContainsHelpFlag(args) {
		PrintCreateHelp(os.Stdout)
		return 0
	}

	opts, err := ParseCreateArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}

	err = validateCreateOptions(opts)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}

	startDir, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		return 1
	}

	targFile, err := FindOrCreateTargFile(startDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error finding/creating targ file: %v\n", err)
		return 1
	}

	err = AddTargetToFileWithOptions(targFile, opts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error adding target: %v\n", err)
		return 1
	}

	fullPath := append(opts.Path, opts.Name) //nolint:gocritic // intentional copy
	fmt.Printf("Created target %q in %s\n", strings.Join(fullPath, "/"), targFile)

	return 0
}

func (r *targRunner) handleEarlyFlags() (exitCode int, done bool) {
	seenTarget := false
	helpSeen := false

	for i, arg := range r.args {
		if !strings.HasPrefix(arg, "-") && !seenTarget {
			seenTarget = true
		}

		f := flags.Find(arg)
		if f == nil {
			continue
		}

		if f.Removed != "" {
			_, _ = fmt.Fprintf(os.Stderr, "%s: %s\n", arg, f.Removed)
			return 1, true
		}

		if f.Long == "help" {
			helpSeen = true
		}

		if !seenTarget {
			remaining := r.args[i+1:]
			if helpSeen {
				remaining = append([]string{helpLong}, remaining...)
			}

			if code, handled := r.dispatchFlagCommand(f.Long, remaining); handled {
				return code, true
			}
		}
	}

	return 0, false
}

func (r *targRunner) handleIsolatedModule(infos []discover.PackageInfo) int {
	// Create isolated build directory with copied files
	dep := resolveTargDependency()

	isolatedDir, cleanup, err := createIsolatedBuildDir(infos, r.startDir, dep)
	if err != nil {
		r.logError("Error creating isolated build directory", err)
		return r.exitWithCleanup(1)
	}

	defer cleanup()

	// Remap package infos to point to isolated directory
	isolatedInfos, _, err := remapPackageInfosToIsolated(infos, r.startDir, isolatedDir)
	if err != nil {
		r.logError("Error remapping package infos", err)
		return r.exitWithCleanup(1)
	}

	bootstrap, err := r.prepareBootstrap(
		isolatedInfos,
		isolatedDir,
		isolatedModuleName,
	)
	if err != nil {
		r.logError("", err)
		return r.exitWithCleanup(1)
	}

	// Use startDir for cache key computation to enable caching across runs
	binaryPath, err := r.setupBinaryPath(r.startDir, bootstrap.cacheKey)
	if err != nil {
		r.logError("Error creating cache directory", err)
		return r.exitWithCleanup(1)
	}

	targBinName := extractBinName(r.binArg)

	// Try cached binary first
	if !r.noBinaryCache {
		if code, ran := r.tryRunCached(binaryPath, targBinName); ran {
			return code
		}
	}

	// Build and run from isolated directory
	return r.buildAndRunIsolated(isolatedDir, binaryPath, targBinName, bootstrap.code)
}

func (r *targRunner) handleMultiModule(
	moduleGroups []moduleTargets,
	helpRequested, helpTargets bool,
) int {
	registry, err := buildMultiModuleBinaries(
		moduleGroups,
		r.startDir,
		r.noBinaryCache,
		r.errOut,
	)
	if err != nil {
		r.logError("Error building module binaries", err)
		return r.exitWithCleanup(1)
	}

	if helpRequested && !helpTargets {
		printMultiModuleHelp(registry)
		return 0
	}

	err = dispatchCommand(registry, r.args, r.errOut, r.binArg)
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}

		r.logError("Error", err)

		return 1
	}

	return 0
}

func (r *targRunner) handleNoTargets() int {
	if len(r.args) > 0 && r.args[0] == completeCommand {
		printNoTargetsCompletion(r.args)
		return 0
	}

	r.logError("Error: no target files found", nil)

	return r.exitWithCleanup(1)
}

func (r *targRunner) handleSingleModule(infos []discover.PackageInfo) int {
	filePaths := collectFilePaths(infos)

	if len(filePaths) == 0 {
		return r.handleNoTargets()
	}

	_, _, moduleFound, err := FindModuleForPath(filePaths[0])
	if err != nil {
		r.logError("Error checking for module", err)
		return r.exitWithCleanup(1)
	}

	// Use isolated build when no module found
	if !moduleFound {
		return r.handleIsolatedModule(infos)
	}

	importRoot, modulePath, _, err := FindModuleForPath(filePaths[0])
	if err != nil {
		r.logError("Error checking for module", err)
		return r.exitWithCleanup(1)
	}

	bootstrap, err := r.prepareBootstrap(infos, importRoot, modulePath)
	if err != nil {
		r.logError("", err)
		return r.exitWithCleanup(1)
	}

	binaryPath, err := r.setupBinaryPath(importRoot, bootstrap.cacheKey)
	if err != nil {
		r.logError("Error creating cache directory", err)
		return r.exitWithCleanup(1)
	}

	targBinName := extractBinName(r.binArg)

	// Try cached binary first
	if !r.noBinaryCache {
		if code, ran := r.tryRunCached(binaryPath, targBinName); ran {
			return code
		}
	}

	// Build and run
	return r.buildAndRun(importRoot, binaryPath, targBinName, bootstrap.code)
}

func (r *targRunner) handleSyncFlag(args []string) int {
	if ContainsHelpFlag(args) {
		PrintSyncHelp(os.Stdout)
		return 0
	}

	opts, err := ParseSyncArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}

	startDir, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		return 1
	}

	targFile, err := FindOrCreateTargFile(startDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error finding/creating targ file: %v\n", err)
		return 1
	}

	exists, err := CheckImportExists(targFile, opts.PackagePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error checking imports: %v\n", err)
		return 1
	}

	if exists {
		_, _ = fmt.Fprintf(os.Stderr, "%v: %s\n", errPackageAlreadySynced, opts.PackagePath)
		return 1
	}

	err = fetchPackage(opts.PackagePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to fetch package: %v\n", err)
		return 1
	}

	err = AddImportToTargFile(targFile, opts.PackagePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error adding import: %v\n", err)
		return 1
	}

	fmt.Printf("Synced %q to %s\n", opts.PackagePath, targFile)
	fmt.Println()
	fmt.Println("All targets from this package are deregistered by default to prevent conflicts.")
	fmt.Println("To use them, edit", targFile+":")
	fmt.Println("  - Remove the DeregisterFrom line to use all targets")
	fmt.Println("  - Or selectively re-register: targ.Register(pkg.TargetName)")

	return 0
}

func (r *targRunner) handleToFuncFlag(args []string) int {
	if ContainsHelpFlag(args) {
		PrintToFuncHelp(os.Stdout)
		return 0
	}

	if len(args) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "--to-func requires a target name")
		return 1
	}

	return convertTargetInFiles(
		"--to-func",
		args[0],
		"function",
		"a string target",
		ConvertStringTargetToFunc,
	)
}

func (r *targRunner) handleToStringFlag(args []string) int {
	if ContainsHelpFlag(args) {
		PrintToStringHelp(os.Stdout)
		return 0
	}

	if len(args) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "--to-string requires a target name")
		return 1
	}

	return convertTargetInFiles(
		"--to-string",
		args[0],
		"string",
		"a function target",
		ConvertFuncTargetToString,
	)
}

func (r *targRunner) logError(prefix string, err error) {
	switch {
	case prefix != "" && err != nil:
		_, _ = fmt.Fprintf(r.errOut, "%s: %v\n", prefix, err)
	case prefix != "":
		_, _ = fmt.Fprintln(r.errOut, prefix)
	case err != nil:
		_, _ = fmt.Fprintf(r.errOut, "%v\n", err)
	}
}

func (r *targRunner) prepareBootstrap(
	infos []discover.PackageInfo,
	importRoot, modulePath string,
) (moduleBootstrap, error) {
	data, err := buildBootstrapData(infos, importRoot, modulePath)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error preparing bootstrap: %w", err)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error generating code: %w", err)
	}

	taggedFiles, err := discover.TaggedFiles(osFileSystem{}, discover.Options{
		StartDir: r.startDir,
		BuildTag: buildTag,
	})
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error gathering tagged files: %w", err)
	}

	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error gathering module files: %w", err)
	}

	cacheInputs := slices.Concat(taggedFiles, moduleFiles)

	cacheKey, err := computeCacheKey(modulePath, importRoot, "targ", buf.Bytes(), cacheInputs)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("error computing cache key: %w", err)
	}

	return moduleBootstrap{code: buf.Bytes(), cacheKey: cacheKey}, nil
}

func (r *targRunner) resolveBootstrapDir(buildDir string, isolated bool) string {
	if isolated {
		return buildDir
	}

	projCache := projectCacheDir(buildDir)

	return filepath.Join(projCache, "tmp")
}

func (r *targRunner) run() int {
	// Handle removed flags early (--init, --alias, --move)
	if code, done := r.handleEarlyFlags(); done {
		return code
	}

	// Setup quiet mode for completion
	if len(r.args) > 0 && r.args[0] == completeCommand {
		r.errOut = io.Discard
	}

	helpRequested, helpTargets := ParseHelpRequest(r.args)

	var flags TargFlags

	flags, r.args = ExtractTargFlags(r.args)
	r.noBinaryCache = flags.NoBinaryCache

	var err error

	// Use source dir if provided, otherwise use current working directory
	if flags.SourceDir != "" {
		r.startDir, err = r.validateSourceDir(flags.SourceDir)
		if err != nil {
			r.logError("Error with --source directory", err)
			return 1
		}
	} else {
		r.startDir, err = os.Getwd()
		if err != nil {
			r.logError("Error resolving working directory", err)
			return 1
		}
	}

	// Discover targ packages
	infos, err := r.discoverPackages()
	if err != nil {
		r.logError("", err)
		return 1
	}

	// Group packages by module
	moduleGroups, err := groupByModule(infos, r.startDir)
	if err != nil {
		r.logError("Error grouping packages by module", err)
		return r.exitWithCleanup(1)
	}

	// Handle multi-module cases
	if len(moduleGroups) > 1 {
		return r.handleMultiModule(moduleGroups, helpRequested, helpTargets)
	}

	// Single module case
	return r.handleSingleModule(infos)
}

func (r *targRunner) setupBinaryPath(importRoot, cacheKey string) (string, error) {
	projCache := projectCacheDir(importRoot)

	cacheDir := filepath.Join(projCache, "bin")

	//nolint:gosec,mnd // standard cache directory permissions
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	return filepath.Join(cacheDir, "targ_"+cacheKey), nil
}

func (r *targRunner) tryRunCached(binaryPath, targBinName string) (exitCode int, ran bool) {
	info, err := os.Stat(binaryPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return 0, false
	}

	//nolint:gosec // build tool runs cached binary by design
	cmd := exec.CommandContext(context.Background(), binaryPath, r.args...)

	cmd.Env = append(os.Environ(), "TARG_BIN_NAME="+targBinName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = r.errOut
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return r.exitWithCleanup(exitErr.ExitCode()), true
		}

		r.logError("Error running command", err)

		return r.exitWithCleanup(1), true
	}

	return 0, true
}

func (r *targRunner) validateSourceDir(path string) (string, error) {
	// Convert to absolute path if relative
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			//nolint:err113 // specific user-facing error with path context
			return "", fmt.Errorf("directory does not exist: %s", absPath)
		}

		return "", fmt.Errorf("checking path: %w", err)
	}

	if !info.IsDir() {
		//nolint:err113 // specific user-facing error with path context
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}

	return absPath, nil
}

// AddTargetToFileWithFileOps adds a target using injected file operations.
type targetCodeResult struct {
	code            string
	needsTimeImport bool
}

// targetConverter converts a target in a file. Returns true if conversion was performed.
type targetConverter func(filePath, targetName string) (bool, error)

// addBlankImport adds a blank import (`_ "pkg"`) to the file's import declarations.
func addBlankImport(file *ast.File, packagePath string) {
	importSpec := &ast.ImportSpec{
		Name: ast.NewIdent("_"),
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: strconv.Quote(packagePath),
		},
	}

	// Find or create import declaration
	var importDecl *ast.GenDecl

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl

			break
		}
	}

	if importDecl != nil {
		importDecl.Specs = append(importDecl.Specs, importSpec)
	} else {
		importDecl = &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{importSpec},
		}
		file.Decls = append([]ast.Decl{importDecl}, file.Decls...)
	}
}

// addDeregisterFromToInit adds a `_ = targ.DeregisterFrom("pkg")` call to init().
// If init() doesn't exist, it creates one. The call is prepended to the body.
func addDeregisterFromToInit(file *ast.File, packagePath string) {
	// Build the statement: _ = targ.DeregisterFrom("pkg")
	deregisterStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("_")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("targ"),
					Sel: ast.NewIdent("DeregisterFrom"),
				},
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.STRING,
						Value: strconv.Quote(packagePath),
					},
				},
			},
		},
	}

	// Find existing init() function
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != "init" || funcDecl.Recv != nil {
			continue
		}

		// Found init() — append the DeregisterFrom call at the end
		funcDecl.Body.List = append(funcDecl.Body.List, deregisterStmt)

		return
	}

	// No init() found — create one
	initFunc := &ast.FuncDecl{
		Name: ast.NewIdent("init"),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{deregisterStmt},
		},
	}

	// Insert after imports (find last import decl position)
	insertIdx := 0

	for i, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			insertIdx = i + 1
		}
	}

	// Insert at the found position
	file.Decls = slices.Insert(file.Decls, insertIdx, ast.Decl(initFunc))
}

func addRegisterArgToInit(content, registerVar string) (string, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parsing file: %w", err)
	}

	call := findRegisterCall(file)
	if call == nil {
		// No init() with targ.Register() found - create one
		initBlock := fmt.Sprintf("\nfunc init() {\n\ttarg.Register(%s)\n}\n", registerVar)
		return content + initBlock, nil
	}

	if registerArgExists(call.Args, registerVar) {
		return content, nil
	}

	call.Args = append(call.Args, ast.NewIdent(registerVar))

	var buf bytes.Buffer

	err = format.Node(&buf, fset, file)
	if err != nil {
		return "", fmt.Errorf("formatting file: %w", err)
	}

	return buf.String(), nil
}

// addTimeImport adds "time" to the import block in generated Go source.
func addTimeImport(content string) string {
	// Case 1: single import — convert to grouped
	const singleImport = `import "github.com/toejough/targ"`
	if strings.Contains(content, singleImport) {
		return strings.Replace(content, singleImport,
			"import (\n\t\"time\"\n\n\t\"github.com/toejough/targ\"\n)", 1)
	}

	// Case 2: grouped import with targ — insert "time" before the targ line
	if before, _, ok := strings.Cut(content, `"github.com/toejough/targ"`); ok {
		insertAt := strings.LastIndex(before, "\n") + 1
		return content[:insertAt] + "\t\"time\"\n\n" + content[insertAt:]
	}

	return content
}

// backoffToGoCode converts "duration,multiplier" to Go code like "1*time.Second, 2.0".
func backoffToGoCode(s string) (string, error) {
	parts := strings.SplitN(s, ",", 2) //nolint:mnd // format is "duration,multiplier"
	if len(parts) != 2 {               //nolint:mnd // need exactly 2 parts
		return "", fmt.Errorf("%w: %q", errBackoffFormat, s)
	}

	durCode, err := durationToGoCode(parts[0])
	if err != nil {
		return "", err
	}

	mult := strings.TrimSpace(parts[1])

	_, err = strconv.ParseFloat(mult, 64)
	if err != nil {
		return "", fmt.Errorf("invalid multiplier %q: %w", mult, err)
	}

	return durCode + ", " + mult, nil
}

// buildAndQueryBinary builds the binary and queries its commands.
func buildAndQueryBinary(
	ctx buildContext,
	_ moduleTargets,
	dep TargDependency,
	binaryPath string,
	bootstrap moduleBootstrap,
	errOut io.Writer,
) ([]commandInfo, error) {
	bootstrapDir := filepath.Join(projectCacheDir(ctx.importRoot), "tmp")
	if ctx.usingFallback {
		bootstrapDir = filepath.Join(ctx.buildRoot, "tmp")
	}

	tempFile, cleanupTemp, err := WriteBootstrapFile(bootstrapDir, bootstrap.code)
	if err != nil {
		return nil, fmt.Errorf("writing bootstrap file: %w", err)
	}

	defer func() { _ = cleanupTemp() }()

	ensureTargDependency(dep, ctx.importRoot)

	err = runGoBuild(ctx, binaryPath, tempFile, errOut)
	if err != nil {
		return nil, err
	}

	cmds, err := queryModuleCommands(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("querying commands: %w", err)
	}

	return cmds, nil
}

func buildBootstrapData(
	infos []discover.PackageInfo,
	moduleRoot string,
	modulePath string,
) (bootstrapData, error) {
	builder := newBootstrapBuilder(moduleRoot, modulePath)

	for _, info := range infos {
		err := builder.processPackage(info)
		if err != nil {
			return bootstrapData{}, err
		}
	}

	return builder.buildResult(), nil
}

// buildDepsCode generates a single .Deps() call for the target's dependencies.
//
// Note: This generates single-group deps (e.g., .Deps(A, B, targ.DepModeParallel)).
// Chained dependency groups (e.g., .Deps(A).Deps(B, C, parallel).Deps(D)) are not
// supported in code generation since targ create is designed for simple targets.
// Users needing chained groups can manually edit the generated code.
func buildDepsCode(opts CreateOptions) string {
	if len(opts.Deps) == 0 {
		return ""
	}

	depVars := make([]string, len(opts.Deps))
	for i, dep := range opts.Deps {
		depVars[i] = KebabToPascal(dep)
	}

	depsArgs := strings.Join(depVars, ", ")
	if opts.DepMode == "parallel" {
		depsArgs += ", targ.DepModeParallel"
	}

	return fmt.Sprintf(".Deps(%s)", depsArgs)
}

// buildModuleBinary builds a single module's binary and queries its commands.
func buildModuleBinary(
	mt moduleTargets,
	startDir string,
	dep TargDependency,
	noBinaryCache bool,
	errOut io.Writer,
) (moduleRegistry, error) {
	reg := moduleRegistry{
		ModuleRoot: mt.ModuleRoot,
		ModulePath: mt.ModulePath,
	}

	err := validateNoPackageMain(mt)
	if err != nil {
		return reg, err
	}

	buildCtx, err := prepareBuildContext(mt, startDir, dep)
	if err != nil {
		return reg, err
	}

	bootstrap, err := generateModuleBootstrap(mt, buildCtx.importRoot)
	if err != nil {
		return reg, err
	}

	binaryPath, err := setupBinaryPath(buildCtx.importRoot, mt.ModulePath, bootstrap)
	if err != nil {
		return reg, err
	}

	reg.BinaryPath = binaryPath

	if !noBinaryCache {
		if cmds, ok := tryCachedBinary(binaryPath); ok {
			reg.Commands = cmds
			return reg, nil
		}
	}

	cmds, err := buildAndQueryBinary(
		buildCtx,
		mt,
		dep,
		binaryPath,
		bootstrap,
		errOut,
	)
	if err != nil {
		return reg, err
	}

	reg.Commands = cmds

	return reg, nil
}

// buildMultiModuleBinaries builds a binary for each module group and returns the registry.
func buildMultiModuleBinaries(
	moduleGroups []moduleTargets,
	startDir string,
	noBinaryCache bool,
	errOut io.Writer,
) ([]moduleRegistry, error) {
	registry := make([]moduleRegistry, 0, len(moduleGroups))

	dep := resolveTargDependency()

	for _, mt := range moduleGroups {
		reg, err := buildModuleBinary(mt, startDir, dep, noBinaryCache, errOut)
		if err != nil {
			return nil, fmt.Errorf("building module %s: %w", mt.ModulePath, err)
		}

		registry = append(registry, reg)
	}

	return registry, nil
}

func buildPatternsCode(method string, patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}

	quoted := make([]string, len(patterns))
	for i, p := range patterns {
		quoted[i] = fmt.Sprintf("%q", p)
	}

	return fmt.Sprintf(".%s(%s)", method, strings.Join(quoted, ", "))
}

func buildSourceRoot() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok || file == "" {
		return "", false
	}

	dir := filepath.Dir(file)
	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return "", false
}

// buildTargetExpression builds just the targ.Targ() expression without var declaration.
func buildTargetExpression(opts CreateOptions) (string, error) {
	var expr strings.Builder

	expr.WriteString(fmt.Sprintf("targ.Targ(%q)", escapeGoString(opts.ShellCmd)))
	expr.WriteString(fmt.Sprintf(".Name(%q)", opts.Name))
	expr.WriteString(buildDepsCode(opts))
	expr.WriteString(buildPatternsCode("Cache", opts.Cache))
	expr.WriteString(buildPatternsCode("Watch", opts.Watch))

	if opts.Timeout != "" {
		durCode, err := durationToGoCode(opts.Timeout)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}

		expr.WriteString(fmt.Sprintf(".Timeout(%s)", durCode))
	}

	if opts.Times > 0 {
		expr.WriteString(fmt.Sprintf(".Times(%d)", opts.Times))
	}

	if opts.Retry {
		expr.WriteString(".Retry()")
	}

	if opts.Backoff != "" {
		backoffCode, err := backoffToGoCode(opts.Backoff)
		if err != nil {
			return "", fmt.Errorf("invalid backoff: %w", err)
		}

		expr.WriteString(fmt.Sprintf(".Backoff(%s)", backoffCode))
	}

	return expr.String(), nil
}

func buildTargetCode(varName string, opts CreateOptions) (targetCodeResult, error) {
	var code strings.Builder

	needsTimeImport := false

	code.WriteString(fmt.Sprintf("\n// %s runs: %s\n", varName, opts.ShellCmd))
	code.WriteString(fmt.Sprintf("var %s = targ.Targ(%q)", varName, escapeGoString(opts.ShellCmd)))
	code.WriteString(fmt.Sprintf(".Name(%q)", opts.Name))
	code.WriteString(buildDepsCode(opts))
	code.WriteString(buildPatternsCode("Cache", opts.Cache))
	code.WriteString(buildPatternsCode("Watch", opts.Watch))

	if opts.Timeout != "" {
		durCode, err := durationToGoCode(opts.Timeout)
		if err != nil {
			return targetCodeResult{}, fmt.Errorf("invalid timeout: %w", err)
		}

		code.WriteString(fmt.Sprintf(".Timeout(%s)", durCode))

		needsTimeImport = true
	}

	if opts.Times > 0 {
		code.WriteString(fmt.Sprintf(".Times(%d)", opts.Times))
	}

	if opts.Retry {
		code.WriteString(".Retry()")
	}

	if opts.Backoff != "" {
		backoffCode, err := backoffToGoCode(opts.Backoff)
		if err != nil {
			return targetCodeResult{}, fmt.Errorf("invalid backoff: %w", err)
		}

		code.WriteString(fmt.Sprintf(".Backoff(%s)", backoffCode))

		needsTimeImport = true
	}

	code.WriteString("\n")

	return targetCodeResult{code: code.String(), needsTimeImport: needsTimeImport}, nil
}

// cleanupStaleModSymlinks removes stale go.mod/go.sum symlinks from before the fix.
func cleanupStaleModSymlinks(root string) {
	for _, name := range []string{"go.mod", "go.sum"} {
		dst := filepath.Join(root, name)
		if symlinkExists(dst) {
			_ = os.Remove(dst)
		}
	}
}

func collectFilePaths(infos []discover.PackageInfo) []string {
	// Count total files for preallocation
	totalFiles := 0
	for _, info := range infos {
		totalFiles += len(info.Files)
	}

	filePaths := make([]string, 0, totalFiles)

	for _, info := range infos {
		for _, f := range info.Files {
			filePaths = append(filePaths, f.Path)
		}
	}

	return filePaths
}

// ParseCreateArgs parses arguments after --create into CreateOptions.
// Format: [path...] <name> [--deps dep1 dep2...] [--cache/--watch pattern1...] [flags...] "shell-command"
// collectListArg collects non-flag arguments into a slice until the next flag or end.
func collectListArg(remaining []string, i int) ([]string, int) {
	var values []string
	for i < len(remaining) && !strings.HasPrefix(remaining[i], "--") {
		values = append(values, remaining[i])
		i++
	}

	return values, i
}

func collectModuleFiles(moduleRoot string) ([]discover.TaggedFile, error) {
	var files []discover.TaggedFile

	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking directory: %w", err)
		}

		if entry.IsDir() {
			return skipIfVendorOrGit(entry.Name())
		}

		if !isIncludableModuleFile(entry.Name()) {
			return nil
		}

		//nolint:gosec // build tool reads source files by design
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", path, err)
		}

		files = append(files, discover.TaggedFile{
			Path:    path,
			Content: data,
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking module directory: %w", err)
	}

	return files, nil
}

// collectModuleTaggedFiles collects tagged files from a module's packages.
func collectModuleTaggedFiles(mt moduleTargets) ([]discover.TaggedFile, error) {
	var files []discover.TaggedFile

	for _, pkg := range mt.Packages {
		for _, f := range pkg.Files {
			data, err := os.ReadFile(f.Path)
			if err != nil {
				return nil, fmt.Errorf("reading tagged file %s: %w", f.Path, err)
			}

			files = append(files, discover.TaggedFile{
				Path:    f.Path,
				Content: data,
			})
		}
	}

	return files, nil
}

func collectSortedCommands(registry []moduleRegistry) []cmdEntry {
	totalCmds := 0
	for _, reg := range registry {
		totalCmds += len(reg.Commands)
	}

	allCmds := make([]cmdEntry, 0, totalCmds)

	for _, reg := range registry {
		for _, cmd := range reg.Commands {
			allCmds = append(allCmds, cmdEntry{cmd.Name, cmd.Description})
		}
	}

	sort.Slice(allCmds, func(i, j int) bool { return allCmds[i].name < allCmds[j].name })

	return allCmds
}

func commonPrefix(a, b []string) []string {
	limit := min(len(b), len(a))

	for i := range limit {
		if a[i] != b[i] {
			return a[:i]
		}
	}

	return a[:limit]
}

func compressNamespacePaths(paths map[string][]string) map[string][]string {
	root := &namespaceNode{Children: make(map[string]*namespaceNode)}
	out := make(map[string][]string, len(paths))

	for file, parts := range paths {
		if len(parts) == 0 {
			out[file] = nil
			continue
		}

		root.insertPath(file, parts)
	}

	root.collectCompressedPaths(out, nil, true)

	return out
}

func computeCacheKey(
	modulePath, moduleRoot, buildTag string,
	bootstrap []byte,
	tagged []discover.TaggedFile,
) (string, error) {
	hasher := sha256.New()
	write := func(value string) {
		hasher.Write([]byte(value))
		hasher.Write([]byte{0})
	}
	write("module:" + modulePath)
	write("root:" + moduleRoot)
	write("tag:" + buildTag)
	write("bootstrap:")
	hasher.Write(bootstrap)
	hasher.Write([]byte{0})

	sort.Slice(tagged, func(i, j int) bool {
		return tagged[i].Path < tagged[j].Path
	})

	for _, file := range tagged {
		if !utf8.ValidString(file.Path) {
			return "", fmt.Errorf("%w: %q", errInvalidUTF8Path, file.Path)
		}

		write("file:" + file.Path)
		hasher.Write(file.Content)
		hasher.Write([]byte{0})
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// computeModuleCacheKey computes the cache key for a module build.
func computeModuleCacheKey(mt moduleTargets, importRoot string, bootstrap []byte) (string, error) {
	taggedFiles, err := collectModuleTaggedFiles(mt)
	if err != nil {
		return "", fmt.Errorf("gathering tagged files: %w", err)
	}

	moduleFiles, err := collectModuleFiles(importRoot)
	if err != nil {
		return "", fmt.Errorf("gathering module files: %w", err)
	}

	cacheInputs := slices.Concat(taggedFiles, moduleFiles)

	cacheKey, err := computeCacheKey(mt.ModulePath, importRoot, "targ", bootstrap, cacheInputs)
	if err != nil {
		return "", fmt.Errorf("computing cache key: %w", err)
	}

	return cacheKey, nil
}

// convertTargetInFiles finds and converts a target using the provided converter.
func convertTargetInFiles(
	_, targetName, successVerb, notFoundDesc string,
	convert targetConverter,
) int {
	startDir, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		return 1
	}

	targFiles, err := findTargFiles(startDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error finding targ files: %v\n", err)
		return 1
	}

	if len(targFiles) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "no targ files found")
		return 1
	}

	for _, targFile := range targFiles {
		ok, convErr := convert(targFile, targetName)
		if convErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error converting target: %v\n", convErr)
			return 1
		}

		if ok {
			fmt.Printf("Converted target %q to %s in %s\n", targetName, successVerb, targFile)
			return 0
		}
	}

	_, _ = fmt.Fprintf(os.Stderr, "target %q not found or not %s\n", targetName, notFoundDesc)

	return 1
}

// copyFileStrippingTag copies a file to destPath, removing the //go:build targ line.
func copyFileStrippingTag(srcPath, destPath string) error {
	//nolint:gosec // build tool reads source files by design
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading source file: %w", err)
	}

	content := stripBuildTag(string(data))

	err = os.WriteFile(destPath, []byte(content), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing destination file: %w", err)
	}

	return nil
}

// createIsolatedBuildDir creates an isolated build directory with targ files.
// Files are copied (with build tags stripped) preserving collapsed namespace paths.
// Returns the tmp directory path, the module path to use for imports, and a cleanup function.
func createIsolatedBuildDir(
	infos []discover.PackageInfo,
	startDir string,
	dep TargDependency,
) (tmpDir string, cleanup func(), err error) {
	filePaths := collectFilePaths(infos)

	paths, err := NamespacePaths(filePaths, startDir)
	if err != nil {
		return "", nil, fmt.Errorf("computing namespace paths: %w", err)
	}

	tmpDir, err = os.MkdirTemp("", "targ-build-")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp directory: %w", err)
	}

	cleanup = func() {
		_ = os.RemoveAll(tmpDir)
	}

	// Copy files using collapsed namespace paths
	for _, info := range infos {
		for _, f := range info.Files {
			collapsedPath := paths[f.Path]

			var targetDir string

			if len(collapsedPath) > 0 {
				// Use all but the last element (which is the filename stem)
				dirParts := collapsedPath[:len(collapsedPath)-1]
				// Add the package name as the final directory
				dirParts = append(dirParts, info.Package)
				targetDir = filepath.Join(tmpDir, filepath.Join(dirParts...))
			} else {
				targetDir = filepath.Join(tmpDir, info.Package)
			}

			//nolint:gosec,mnd // standard directory permissions
			err := os.MkdirAll(targetDir, 0o755)
			if err != nil {
				cleanup()
				return "", nil, fmt.Errorf("creating target directory: %w", err)
			}

			destPath := filepath.Join(targetDir, filepath.Base(f.Path))

			err = copyFileStrippingTag(f.Path, destPath)
			if err != nil {
				cleanup()
				return "", nil, fmt.Errorf("copying file %s: %w", f.Path, err)
			}
		}
	}

	// Create synthetic go.mod
	err = writeIsolatedGoMod(tmpDir, dep)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return tmpDir, cleanup, nil
}

// dispatchCommand finds the right binary for a command and executes it.
func dispatchCommand(
	registry []moduleRegistry,
	args []string,
	errOut io.Writer,
	binArg string,
) error {
	if isHelpRequest(args) {
		printMultiModuleHelp(registry)
		return nil
	}

	if len(args) > 0 && args[0] == completeCommand {
		return dispatchCompletion(registry, args)
	}

	cmdName := args[0]
	if binaryPath, ok := findCommandBinary(registry, cmdName); ok {
		return runModuleBinary(binaryPath, args, errOut, binArg)
	}

	_, _ = fmt.Fprintf(errOut, "Unknown command: %s\n", cmdName)

	printMultiModuleHelp(registry)

	return fmt.Errorf("%w: %s", errUnknownCommand, cmdName)
}

// dispatchCompletion handles completion requests by querying all binaries.
func dispatchCompletion(registry []moduleRegistry, args []string) error {
	if len(args) < minArgsForCompletion {
		return nil
	}

	// Query each binary for completions and aggregate
	seen := make(map[string]bool)

	for _, reg := range registry {
		//nolint:gosec // build tool runs module binaries by design
		cmd := exec.CommandContext(context.Background(), reg.BinaryPath, args...)

		output, err := cmd.Output()
		if err != nil {
			continue // Skip failed completions
		}

		for line := range strings.SplitSeq(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				fmt.Println(line)
			}
		}
	}

	return nil
}

// durationToGoCode converts a duration string like "30s" to Go code like "30 * time.Second".
func durationToGoCode(s string) (string, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return "", fmt.Errorf("parsing duration %q: %w", s, err)
	}

	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%d * time.Hour", int(d/time.Hour)), nil
	case d%time.Minute == 0:
		return fmt.Sprintf("%d * time.Minute", int(d/time.Minute)), nil
	case d%time.Second == 0:
		return fmt.Sprintf("%d * time.Second", int(d/time.Second)), nil
	default:
		return fmt.Sprintf("time.Duration(%d)", d.Nanoseconds()), nil
	}
}

// ensureTargDependency runs go get to ensure targ dependency is available.
func ensureTargDependency(dep TargDependency, importRoot string) {
	//nolint:gosec // build tool runs go get by design
	getCmd := exec.CommandContext(context.Background(), "go", "get", dep.ModulePath)
	getCmd.Dir = importRoot
	getCmd.Stdout = io.Discard
	getCmd.Stderr = io.Discard
	_ = getCmd.Run()
}

// ensureTargImport ensures "github.com/toejough/targ" is in the import block.
func ensureTargImport(file *ast.File) {
	const targPkg = `"github.com/toejough/targ"`

	for _, imp := range file.Imports {
		if imp.Path.Value == targPkg {
			return // Already imported
		}
	}

	// Not found — add it
	importSpec := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: targPkg,
		},
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if ok && genDecl.Tok == token.IMPORT {
			// Prepend to existing import block
			genDecl.Specs = append([]ast.Spec{importSpec}, genDecl.Specs...)
			return
		}
	}

	// No import block — create one
	importDecl := &ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: []ast.Spec{importSpec},
	}

	file.Decls = append([]ast.Decl{importDecl}, file.Decls...)
}

// escapeGoString escapes a string for use in a Go string literal.
func escapeGoString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")

	return s
}

func extractBinName(binArg string) string {
	if binArg == "" {
		return buildTag
	}

	if idx := strings.LastIndex(binArg, "/"); idx != -1 {
		return binArg[idx+1:]
	}

	if idx := strings.LastIndex(binArg, "\\"); idx != -1 {
		return binArg[idx+1:]
	}

	return binArg
}

// extractFuncTargCall finds targ.Targ(funcName) call and returns the ident and call.
func extractFuncTargCall(expr ast.Expr) (*ast.Ident, *ast.CallExpr) {
	// Walk up the chain to find targ.Targ()
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return nil, nil
		}

		// Check if this is targ.Targ()
		if isTargTargCall(call) {
			if len(call.Args) == 1 {
				if ident, ok := call.Args[0].(*ast.Ident); ok {
					return ident, call
				}
			}

			return nil, nil
		}

		// Continue up the chain
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, nil
		}

		expr = sel.X
	}
}

// extractShellCommand finds a function and extracts its sh.Run command.
// Returns empty string if function is not a simple sh.Run call.
//
//nolint:cyclop // AST traversal requires multiple branch checks
func extractShellCommand(file *ast.File, funcName string) (string, *ast.FuncDecl) {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != funcName {
			continue
		}

		// Check for single return statement with sh.Run()
		if len(funcDecl.Body.List) != 1 {
			return "", nil
		}

		retStmt, ok := funcDecl.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(retStmt.Results) != 1 {
			return "", nil
		}

		call, ok := retStmt.Results[0].(*ast.CallExpr)
		if !ok {
			return "", nil
		}

		// Check if it's sh.Run()
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", nil
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok || sel.Sel.Name != "Run" {
			return "", nil
		}

		// Support both sh.Run and targ.Run
		if ident.Name != "sh" && ident.Name != "targ" {
			return "", nil
		}

		// Extract arguments and join into command
		var parts []string

		for _, arg := range call.Args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return "", nil
			}

			part, _ := strconv.Unquote(lit.Value)
			parts = append(parts, part)
		}

		return strings.Join(parts, " "), funcDecl
	}

	return "", nil
}

// extractStringTargCall finds targ.Targ("string") call and returns the string and call.
func extractStringTargCall(expr ast.Expr) (string, *ast.CallExpr) {
	// Walk up the chain to find targ.Targ()
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return "", nil
		}

		// Check if this is targ.Targ()
		if isTargTargCall(call) {
			if len(call.Args) == 1 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					cmd, _ := strconv.Unquote(lit.Value)
					return cmd, call
				}
			}

			return "", nil
		}

		// Continue up the chain
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", nil
		}

		expr = sel.X
	}
}

// fetchPackage runs go get to fetch a package.
func fetchPackage(packagePath string) error {
	cmd := exec.CommandContext(context.Background(), "go", "get", packagePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("go get %s: %w", packagePath, err)
	}

	return nil
}

// findCommandBinary finds the binary path for a command in the registry.
func findCommandBinary(registry []moduleRegistry, cmdName string) (string, bool) {
	for _, reg := range registry {
		for _, cmd := range reg.Commands {
			if cmd.Name == cmdName || strings.HasPrefix(cmd.Name, cmdName+" ") {
				return reg.BinaryPath, true
			}
		}
	}

	return "", false
}

// findFuncTarget searches for a target with a function argument in targ.Targ().
func findFuncTarget(file *ast.File, targetName string) *funcTargetInfo {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || len(valueSpec.Names) == 0 || len(valueSpec.Values) == 0 {
				continue
			}

			if !targetMatchesName(valueSpec, targetName) {
				continue
			}

			ident, call := extractFuncTargCall(valueSpec.Values[0])
			if ident != nil {
				return &funcTargetInfo{call: call, funcIdent: ident}
			}
		}
	}

	return nil
}

// findModCacheDir finds the cached module directory for a clean version.
func findModCacheDir(modulePath, version string) (string, bool) {
	if !isCleanVersion(version) {
		return "", false
	}

	modCache, err := goEnv("GOMODCACHE")
	if err != nil || modCache == "" {
		return "", false
	}

	candidate := filepath.Join(modCache, modulePath+"@"+version)

	statInfo, err := os.Stat(candidate)
	if err == nil && statInfo.IsDir() {
		return candidate, true
	}

	return "", false
}

func findRegisterCall(file *ast.File) *ast.CallExpr {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != "init" || funcDecl.Recv != nil {
			continue
		}

		if call := findRegisterCallInInit(funcDecl); call != nil {
			return call
		}
	}

	return nil
}

func findRegisterCallInInit(funcDecl *ast.FuncDecl) *ast.CallExpr {
	for _, stmt := range funcDecl.Body.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}

		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}

		if isTargRegisterCall(call) {
			return call
		}
	}

	return nil
}

// findStringTarget searches for a target with a string argument in targ.Targ().
func findStringTarget(file *ast.File, targetName string) *stringTargetInfo {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || len(valueSpec.Names) == 0 || len(valueSpec.Values) == 0 {
				continue
			}

			if !targetMatchesName(valueSpec, targetName) {
				continue
			}

			cmd, call := extractStringTargCall(valueSpec.Values[0])
			if cmd != "" {
				return &stringTargetInfo{call: call, shellCmd: cmd}
			}
		}
	}

	return nil
}

func findTargFileInEntries(fileOps FileOps, dir string, entries []fs.DirEntry) (string, bool) {
	for _, entry := range entries {
		if path, ok := targFilePath(fileOps, dir, entry); ok {
			return path, true
		}
	}

	return "", false
}

func findTargFileInTree(fileOps FileOps, dir string) (string, bool, error) {
	entries, err := fileOps.ReadDir(dir)
	if err != nil {
		return "", false, fmt.Errorf("reading directory: %w", err)
	}

	if path, found := findTargFileInEntries(fileOps, dir, entries); found {
		return path, true, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path, found, err := findTargFileInTree(fileOps, filepath.Join(dir, entry.Name()))
		if err != nil {
			return "", false, err
		}

		if found {
			return path, true, nil
		}
	}

	return "", false, nil
}

// findTargFiles finds all files with the targ build tag in the given directory.
func findTargFiles(startDir string) ([]string, error) {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var targFiles []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		path := filepath.Join(startDir, entry.Name())
		if HasTargBuildTag(path) {
			targFiles = append(targFiles, path)
		}
	}

	return targFiles, nil
}

// generateGroupModifications creates group declarations and modifications for the path.
// For existing groups, it returns patches to add the new member.
// For new groups, it returns code to append.
func generateGroupModifications(
	path []string,
	targetVarName, existingContent string,
) groupModifications {
	var mods groupModifications
	if len(path) == 0 {
		return mods
	}

	var newCode strings.Builder

	// Build groups from innermost to outermost
	// e.g., for path ["dev", "lint"] and target "fast":
	// - DevLint group contains DevLintFast (the target)
	// - Dev group contains DevLint
	childVarName := targetVarName

	for i := len(path) - 1; i >= 0; i-- {
		groupPath := path[:i+1]
		groupVarName := PathToPascal(groupPath)
		groupName := path[i] // Use the last component as the group's name

		// Check if group already exists
		groupPattern := fmt.Sprintf("var %s = ", groupVarName)
		if strings.Contains(existingContent, groupPattern) {
			// Group exists - create a patch to add the new member
			patch := CreateGroupMemberPatch(existingContent, groupVarName, childVarName)
			if patch != nil {
				mods.ContentPatches = append(mods.ContentPatches, *patch)
			}

			childVarName = groupVarName

			continue
		}

		newCode.WriteString(fmt.Sprintf("var %s = targ.Group(%q, %s)\n",
			groupVarName, groupName, childVarName))
		childVarName = groupVarName
	}

	mods.newCode = newCode.String()

	return mods
}

// generateModuleBootstrap creates bootstrap code and computes cache key.
func generateModuleBootstrap(
	mt moduleTargets,
	importRoot string,
) (moduleBootstrap, error) {
	data, err := buildBootstrapData(
		mt.Packages,
		importRoot,
		mt.ModulePath,
	)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("preparing bootstrap: %w", err)
	}

	tmpl := template.Must(template.New("main").Parse(bootstrapTemplate))

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return moduleBootstrap{}, fmt.Errorf("generating code: %w", err)
	}

	cacheKey, err := computeModuleCacheKey(mt, importRoot, buf.Bytes())
	if err != nil {
		return moduleBootstrap{}, err
	}

	return moduleBootstrap{
		code:     buf.Bytes(),
		cacheKey: cacheKey,
	}, nil
}

// generateShellFunc generates a function that calls targ.Run with the command.
func generateShellFunc(funcName, shellCmd string) *ast.FuncDecl {
	// Parse command into parts
	parts := strings.Fields(shellCmd)
	if len(parts) == 0 {
		parts = []string{shellCmd}
	}

	// Build arguments for targ.Run
	args := make([]ast.Expr, len(parts))
	for i, part := range parts {
		args[i] = &ast.BasicLit{
			Kind:  token.STRING,
			Value: strconv.Quote(part),
		}
	}

	return &ast.FuncDecl{
		Name: ast.NewIdent(funcName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("error")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X:   ast.NewIdent("targ"),
								Sel: ast.NewIdent("Run"),
							},
							Args: args,
						},
					},
				},
			},
		},
	}
}

func goEnv(key string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "go", "env", key)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting go env %s: %w", key, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// groupByModule groups packages by their module root.
// Packages without a module are grouped under startDir with "targ.local" module path.
func groupByModule(infos []discover.PackageInfo, startDir string) ([]moduleTargets, error) {
	byModule := make(map[string]*moduleTargets)

	for _, info := range infos {
		if len(info.Files) == 0 {
			continue
		}

		// Find module for first file in package
		modRoot, modPath, found, err := FindModuleForPath(info.Files[0].Path)
		if err != nil {
			return nil, err
		}

		if !found {
			// No module found - use startDir as pseudo-module
			modRoot = startDir
			modPath = targLocalModule
		}

		// Group by module root
		if mt, ok := byModule[modRoot]; ok {
			mt.Packages = append(mt.Packages, info)
		} else {
			byModule[modRoot] = &moduleTargets{
				ModuleRoot: modRoot,
				ModulePath: modPath,
				Packages:   []discover.PackageInfo{info},
			}
		}
	}

	// Convert to sorted slice for deterministic ordering
	result := make([]moduleTargets, 0, len(byModule))
	for _, mt := range byModule {
		result = append(result, *mt)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ModuleRoot < result[j].ModuleRoot
	})

	return result, nil
}

// hasNameMethod checks if the expression chain contains .Name("targetName").
func hasNameMethod(expr ast.Expr, targetName string) bool {
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return false
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}

		if sel.Sel.Name == "Name" && len(call.Args) == 1 {
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				name, _ := strconv.Unquote(lit.Value)
				if name == targetName {
					return true
				}
			}
		}

		expr = sel.X
	}
}

// isCleanVersion returns true if the version is suitable for cache lookup.
func isCleanVersion(version string) bool {
	return version != "" && version != "(devel)" && !strings.Contains(version, "+dirty")
}

// isHelpRequest returns true if args represent a help request.
func isHelpRequest(args []string) bool {
	return len(args) == 0 || args[0] == helpShort || args[0] == helpLong
}

// isIncludableModuleFile returns true if the file should be included in module cache.
func isIncludableModuleFile(name string) bool {
	// Include go.mod and go.sum for cache invalidation when dependencies change
	if name == "go.mod" || name == "go.sum" {
		return true
	}

	// Include non-test .go files
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func isTargRegisterCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok || pkgIdent.Name != buildTag || selector.Sel.Name != "Register" {
		return false
	}

	return true
}

// isTargTargCall checks if call is targ.Targ().
func isTargTargCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "targ" && sel.Sel.Name == "Targ"
}

// linkModuleEntry creates a symlink for a single directory entry if needed.
func linkModuleEntry(startDir, root string, entry os.DirEntry) error {
	name := entry.Name()
	// Skip .git and module files - we'll create our own go.mod/go.sum
	if name == ".git" || name == "go.mod" || name == "go.sum" {
		return nil
	}

	src := filepath.Join(startDir, name)
	dst := filepath.Join(root, name)

	if symlinkExists(dst) {
		return nil
	}

	// Remove non-symlink file/dir if it exists
	_ = os.RemoveAll(dst)

	err := os.Symlink(src, dst)
	if err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", dst, src, err)
	}

	return nil
}

func linkModuleRoot(startDir, root string) error {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return fmt.Errorf("reading start directory: %w", err)
	}

	for _, entry := range entries {
		err := linkModuleEntry(startDir, root, entry)
		if err != nil {
			return err
		}
	}

	cleanupStaleModSymlinks(root)

	return nil
}

func looksLikeModulePath(path string) bool {
	if path == "" {
		return false
	}

	first := strings.Split(path, "/")[0]

	return strings.Contains(first, ".")
}

func newBootstrapBuilder(moduleRoot, modulePath string) *bootstrapBuilder {
	return &bootstrapBuilder{
		moduleRoot: moduleRoot,
		modulePath: modulePath,
	}
}

func parseModulePath(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}

	return ""
}

// parseSingleValueArg parses a single-value flag, returning the value and new index.
func parseSingleValueArg(remaining []string, i int, flagName string) (string, int, error) {
	if i >= len(remaining) {
		return "", i, fmt.Errorf("%w: %s requires a value", errCreateUsage, flagName)
	}

	return remaining[i], i + 1, nil
}

// prepareBuildContext determines build roots and handles fallback module setup.
func prepareBuildContext(
	mt moduleTargets,
	startDir string,
	dep TargDependency,
) (buildContext, error) {
	ctx := buildContext{
		usingFallback: mt.ModulePath == targLocalModule,
		buildRoot:     mt.ModuleRoot,
		importRoot:    mt.ModuleRoot,
	}

	if ctx.usingFallback {
		var err error

		ctx.buildRoot, err = EnsureFallbackModuleRoot(startDir, mt.ModulePath, dep)
		if err != nil {
			return ctx, fmt.Errorf("preparing fallback module: %w", err)
		}
	}

	return ctx, nil
}

func printCommandList(allCmds []cmdEntry) {
	maxLen := minCommandNameWidth
	for _, cmd := range allCmds {
		if len(cmd.name) > maxLen {
			maxLen = len(cmd.name)
		}
	}

	indent := strings.Repeat(" ", helpIndentWidth+maxLen+commandNamePadding+1+commandNamePadding)

	for _, cmd := range allCmds {
		lines := strings.Split(cmd.description, "\n")
		fmt.Printf("    %-*s %s\n", maxLen+commandNamePadding, cmd.name, lines[0])

		for _, line := range lines[1:] {
			fmt.Printf("%s%s\n", indent, line)
		}
	}
}

func printFlagList() {
	for _, f := range flags.VisibleFlags() {
		name := "--" + f.Long
		if f.Short != "" {
			name = fmt.Sprintf("--%s, -%s", f.Long, f.Short)
		}

		fmt.Printf("    %-28s %s\n", name, f.Desc)
	}
}

func printMultiModuleHelp(registry []moduleRegistry) {
	fmt.Println("targ is a build-tool runner that discovers tagged commands and executes them.")
	fmt.Println()
	fmt.Println("Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]")
	fmt.Println()
	fmt.Println("Commands:")
	printCommandList(collectSortedCommands(registry))
	fmt.Println()
	fmt.Println("Flags:")
	printFlagList()
	fmt.Println()
	fmt.Println("More info: https://github.com/toejough/targ#readme")
}

// printNoCommandsHelp prints the help message when no commands are found.

// printNoTargetsCompletion outputs completion suggestions when no target files exist.
// This allows users to discover flags even before creating targets.
func printNoTargetsCompletion(args []string) {
	// Parse the command line from __complete args
	if len(args) < minArgsForCompletion {
		return
	}

	cmdLine := args[1]
	parts := strings.Fields(cmdLine)
	// Remove binary name
	if len(parts) > 0 {
		parts = parts[1:]
	}

	// Determine prefix (what user is typing)
	prefix := ""
	if len(parts) > 0 && !strings.HasSuffix(cmdLine, " ") {
		prefix = parts[len(parts)-1]
	}

	// All visible targ flags available at root level
	for _, f := range flags.VisibleFlags() {
		flag := "--" + f.Long
		if strings.HasPrefix(flag, prefix) {
			fmt.Println(flag)
		}
	}
}

// printRootCommands prints commands that are at the root level (no namespace).

// printSubcommandTree prints the top-level subcommand names.

// projectCacheDir returns a project-specific subdirectory within the targ cache.
// Uses a hash of the project path to isolate projects.
func projectCacheDir(projectPath string) string {
	hash := sha256.Sum256([]byte(projectPath))
	return filepath.Join(targCacheDir(), hex.EncodeToString(hash[:8]))
}

// queryModuleCommands queries a module binary for its available commands.
func queryModuleCommands(binaryPath string) ([]commandInfo, error) {
	cmd := exec.CommandContext(context.Background(), binaryPath, "__list")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running __list: %w", err)
	}

	var result listOutput

	err = json.Unmarshal(output, &result)
	if err != nil {
		return nil, fmt.Errorf("parsing __list output: %w", err)
	}

	return result.Commands, nil
}

func registerArgExists(args []ast.Expr, name string) bool {
	for _, arg := range args {
		if ident, ok := arg.(*ast.Ident); ok && ident.Name == name {
			return true
		}
	}

	return false
}

// remapPackageInfosToIsolated creates new package infos with paths pointing to isolated dir.
// Returns the remapped infos and a mapping from new paths to original paths.
func remapPackageInfosToIsolated(
	infos []discover.PackageInfo,
	startDir, isolatedDir string,
) ([]discover.PackageInfo, map[string]string, error) {
	filePaths := collectFilePaths(infos)

	paths, err := NamespacePaths(filePaths, startDir)
	if err != nil {
		return nil, nil, fmt.Errorf("computing namespace paths: %w", err)
	}

	result := make([]discover.PackageInfo, 0, len(infos))
	pathMapping := make(map[string]string) // newPath -> originalPath

	for _, info := range infos {
		newInfo := discover.PackageInfo{
			Package: info.Package,
			Doc:     info.Doc,
		}

		// Compute new directory based on collapsed paths
		var newDir string

		if len(info.Files) > 0 {
			collapsedPath := paths[info.Files[0].Path]
			if len(collapsedPath) > 0 {
				dirParts := collapsedPath[:len(collapsedPath)-1]
				dirParts = append(dirParts, info.Package)
				newDir = filepath.Join(isolatedDir, filepath.Join(dirParts...))
			} else {
				newDir = filepath.Join(isolatedDir, info.Package)
			}
		}

		newInfo.Dir = newDir

		// Remap file paths
		newFiles := make([]discover.FileInfo, 0, len(info.Files))
		for _, f := range info.Files {
			newPath := filepath.Join(newDir, filepath.Base(f.Path))
			pathMapping[newPath] = f.Path // Track original path
			newFiles = append(newFiles, discover.FileInfo{
				Path: newPath,
				Base: f.Base,
			})
		}

		newInfo.Files = newFiles
		result = append(result, newInfo)
	}

	return result, pathMapping, nil
}

// removeFuncDecl removes a function declaration from file.Decls.
func removeFuncDecl(file *ast.File, funcDecl *ast.FuncDecl) {
	newDecls := make([]ast.Decl, 0, len(file.Decls)-1)

	for _, decl := range file.Decls {
		if decl != funcDecl {
			newDecls = append(newDecls, decl)
		}
	}

	file.Decls = newDecls
}

func resolveTargDependency() TargDependency {
	dep := TargDependency{
		ModulePath: defaultTargModulePath,
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return dep
	}

	if looksLikeModulePath(info.Main.Path) {
		dep.ModulePath = info.Main.Path
	}

	if cacheDir, ok := findModCacheDir(dep.ModulePath, info.Main.Version); ok {
		dep.Version = info.Main.Version
		dep.ReplaceDir = cacheDir
	} else if root, ok := buildSourceRoot(); ok {
		dep.ReplaceDir = root
	}

	return dep
}

// runGoBuild executes the go build command.
func runGoBuild(ctx buildContext, binaryPath, tempFile string, errOut io.Writer) error {
	buildArgs := []string{"build", "-tags", buildTag, "-o", binaryPath}
	if ctx.usingFallback {
		buildArgs = append(buildArgs, "-mod=mod")
	}

	buildArgs = append(buildArgs, tempFile)

	//nolint:gosec // build tool runs go build by design
	buildCmd := exec.CommandContext(context.Background(), "go", buildArgs...)

	var buildOutput bytes.Buffer

	buildCmd.Stdout = io.Discard
	buildCmd.Stderr = &buildOutput

	if ctx.usingFallback {
		buildCmd.Dir = ctx.buildRoot
	} else {
		buildCmd.Dir = ctx.importRoot
	}

	err := buildCmd.Run()
	if err != nil {
		if buildOutput.Len() > 0 {
			_, _ = fmt.Fprint(errOut, buildOutput.String())
		}

		return fmt.Errorf("building command: %w", err)
	}

	return nil
}

// runModuleBinary executes a module binary with the given args.
func runModuleBinary(binaryPath string, args []string, errOut io.Writer, binArg string) error {
	proc := exec.CommandContext(context.Background(), binaryPath, args...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = errOut

	proc.Env = append(os.Environ(), "TARG_BIN_NAME="+extractBinName(binArg))

	err := proc.Run()
	if err != nil {
		return fmt.Errorf("running module binary: %w", err)
	}

	return nil
}

// setupBinaryPath creates cache directory and returns binary path.
func setupBinaryPath(importRoot, _ string, bootstrap moduleBootstrap) (string, error) {
	projCache := projectCacheDir(importRoot)
	cacheDir := filepath.Join(projCache, "bin")

	//nolint:gosec,mnd // standard cache directory permissions
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	return filepath.Join(cacheDir, "targ_"+bootstrap.cacheKey), nil
}

// skipIfVendorOrGit returns SkipDir for .git and vendor directories.
func skipIfVendorOrGit(name string) error {
	if name == ".git" || name == "vendor" {
		return filepath.SkipDir
	}

	return nil
}

// stripBuildTag removes the //go:build targ line from source content.
func stripBuildTag(content string) string {
	var result strings.Builder

	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//go:build") && strings.Contains(trimmed, "targ") {
			continue
		}
		// Also skip legacy +build tag
		if strings.HasPrefix(trimmed, "// +build") && strings.Contains(trimmed, "targ") {
			continue
		}

		result.WriteString(line)
		result.WriteString("\n")
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// symlinkExists returns true if dst is an existing symlink.
func symlinkExists(dst string) bool {
	info, err := os.Lstat(dst)
	if err != nil || info == nil {
		return false
	}

	return info.Mode()&os.ModeSymlink != 0
}

// targCacheDir returns the centralized cache directory for targ following XDG spec.
// Uses $XDG_CACHE_HOME/targ or ~/.cache/targ as fallback.
func targCacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "targ")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to temp directory if home can't be determined
		return filepath.Join(os.TempDir(), "targ-cache")
	}

	return filepath.Join(home, ".cache", "targ")
}

func targFilePath(fileOps FileOps, dir string, entry fs.DirEntry) (string, bool) {
	if entry.IsDir() {
		return "", false
	}

	name := entry.Name()
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return "", false
	}

	path := filepath.Join(dir, name)
	if HasTargBuildTagWithFileOps(fileOps, path) {
		return path, true
	}

	return "", false
}

// targetMatchesName checks if a target variable matches the given CLI name.
func targetMatchesName(spec *ast.ValueSpec, targetName string) bool {
	// Check for .Name("targetName") in the chain
	if hasNameMethod(spec.Values[0], targetName) {
		return true
	}

	// Fall back to variable name conversion
	varName := spec.Names[0].Name
	expectedVar := KebabToPascal(targetName)

	return varName == expectedVar
}

// targetNameToFuncName converts a target name to a function name.
func targetNameToFuncName(name string) string {
	// Convert kebab-case to camelCase (first letter lowercase)
	parts := strings.Split(name, "-")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}

	return strings.Join(parts, "")
}

func touchFile(path string) error {
	err := os.WriteFile(path, []byte{}, filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("touching file %s: %w", path, err)
	}

	return nil
}

// tryCachedBinary checks if a cached binary exists and queries its commands.
func tryCachedBinary(binaryPath string) ([]commandInfo, bool) {
	info, err := os.Stat(binaryPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil, false
	}

	cmds, err := queryModuleCommands(binaryPath)
	if err != nil {
		return nil, false
	}

	return cmds, true
}

// validateCreateOptions validates all names in create options are valid kebab-case.
func validateCreateOptions(opts CreateOptions) error {
	if !IsValidTargetName(opts.Name) {
		return fmt.Errorf(
			"%w %q: must be lowercase letters, numbers, and hyphens",
			errInvalidTarget,
			opts.Name,
		)
	}

	for _, p := range opts.Path {
		if !IsValidTargetName(p) {
			return fmt.Errorf(
				"%w %q: must be lowercase letters, numbers, and hyphens",
				errInvalidGroup,
				p,
			)
		}
	}

	for _, dep := range opts.Deps {
		if !IsValidTargetName(dep) {
			return fmt.Errorf(
				"%w %q: must be lowercase letters, numbers, and hyphens",
				errInvalidDependency,
				dep,
			)
		}
	}

	return nil
}

// validateNoPackageMain ensures no targ files use package main.
func validateNoPackageMain(mt moduleTargets) error {
	for _, pkg := range mt.Packages {
		if pkg.Package == pkgNameMain {
			return fmt.Errorf(
				"%w (found in %s); use a named package instead",
				errPackageMainNotAllowed,
				pkg.Dir,
			)
		}
	}

	return nil
}

func writeFallbackGoMod(root, modulePath string, dep TargDependency) error {
	modPath := filepath.Join(root, "go.mod")

	if dep.ModulePath == "" {
		dep.ModulePath = defaultTargModulePath
	}

	lines := []string{
		"module " + modulePath,
		"",
		"go 1.21",
	}
	if dep.Version != "" {
		lines = append(lines, "", fmt.Sprintf("require %s %s", dep.ModulePath, dep.Version))
	}

	if dep.ReplaceDir != "" {
		lines = append(lines, "", fmt.Sprintf("replace %s => %s", dep.ModulePath, dep.ReplaceDir))
	}

	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(modPath, []byte(content), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	return nil
}

// writeFormattedFile writes an AST back to a file with proper formatting.
func writeFormattedFile(filePath string, fset *token.FileSet, file *ast.File) error {
	var buf bytes.Buffer

	err := format.Node(&buf, fset, file)
	if err != nil {
		return fmt.Errorf("formatting code: %w", err)
	}

	//nolint:gosec,mnd // G306: 0o644 is standard permission for source files
	err = os.WriteFile(filePath, buf.Bytes(), 0o644)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// writeIsolatedGoMod creates a go.mod for isolated builds.
func writeIsolatedGoMod(tmpDir string, dep TargDependency) error {
	modPath := filepath.Join(tmpDir, "go.mod")

	if dep.ModulePath == "" {
		dep.ModulePath = defaultTargModulePath
	}

	lines := []string{
		"module " + isolatedModuleName,
		"",
		"go 1.21",
	}

	// Always add require - use a placeholder version if not specified
	version := dep.Version
	if version == "" {
		version = "v0.0.0"
	}

	lines = append(lines, "", fmt.Sprintf("require %s %s", dep.ModulePath, version))

	if dep.ReplaceDir != "" {
		lines = append(lines, "", fmt.Sprintf("replace %s => %s", dep.ModulePath, dep.ReplaceDir))
	}

	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(modPath, []byte(content), filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing isolated go.mod: %w", err)
	}

	// Touch go.sum file
	sumPath := filepath.Join(tmpDir, "go.sum")

	err = os.WriteFile(sumPath, []byte{}, filePermissionsForCode)
	if err != nil {
		return fmt.Errorf("writing go.sum: %w", err)
	}

	return nil
}
