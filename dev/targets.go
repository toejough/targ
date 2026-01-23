//go:build targ

package dev

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/akedrou/textdiff"
	"github.com/toejough/go-reorder"
	"github.com/toejough/targ"
	"github.com/toejough/targ/file"
	"github.com/toejough/targ/sh"
	"github.com/toejough/testredundancy"
)

func init() {
	targ.Register(
		Check,
		CheckCoverage,
		CheckCoverageForFail,
		CheckForFail,
		CheckNils,
		CheckNilsFix,
		CheckNilsForFail,
		CheckThinAPI,
		Clean,
		Coverage,
		Deadcode,
		DeleteDeadcode,
		FindRedundantTests,
		Fmt,
		Fuzz,
		Generate,
		InstallTools,
		Lint,
		LintFast,
		LintForFail,
		Modernize,
		Mutate,
		ReorderDecls,
		ReorderDeclsCheck,
		Test,
		TestForFail,
		Tidy,
		TodoCheck,
		Watch,
	)
}

// Exported variables.
var (
	Check                = targ.Targ(check).Description("Run all checks & fixes")
	CheckCoverage        = targ.Targ(checkCoverage).Description("Check function coverage")
	CheckCoverageForFail = targ.Targ(checkCoverageForFail).Description("Check coverage (no test run)")
	CheckForFail         = targ.Targ(checkForFail).Description("Run all checks (fail-fast)")
	CheckNils            = targ.Targ(checkNils).Description("Check for nil issues")
	CheckNilsFix         = targ.Targ(checkNilsFix).Description("Fix nil issues")
	CheckNilsForFail     = targ.Targ(checkNilsForFail).Description("Check for nil issues (fail)")
	CheckThinAPI         = targ.Targ(checkThinAPI).Description("Check public API is thin wrappers")
	Clean                = targ.Targ(clean).Description("Clean dev environment")
	Coverage             = targ.Targ(coverage).Description("Display coverage report")
	Deadcode             = targ.Targ(deadcode).Description("Check for dead code")
	DeleteDeadcode       = targ.Targ(deleteDeadcode).Description("Delete dead code")
	FindRedundantTests   = targ.Targ(findRedundantTests).Description("Find redundant tests")
	Fmt                  = targ.Targ(fmtCode).Description("Format codebase")
	Fuzz                 = targ.Targ(fuzz).Description("Run fuzz tests")
	Generate             = targ.Targ(generate).Description("Run go generate")
	InstallTools         = targ.Targ(installTools).Description("Install dev tools")
	Lint                 = targ.Targ(lint).Description("Lint codebase")
	LintFast             = targ.Targ(lintFast).Description("Run fast linters")
	LintForFail          = targ.Targ(lintForFail).Description("Lint for pass/fail")
	Modernize            = targ.Targ(modernize).Description("Modernize codebase")
	Mutate               = targ.Targ(mutate).Description("Run mutation tests")
	ReorderDecls         = targ.Targ(reorderDecls).Description("Reorder declarations")
	ReorderDeclsCheck    = targ.Targ(reorderDeclsCheck).Description("Check declaration order")
	Test                 = targ.Targ(test).Description("Run unit tests")
	TestForFail          = targ.Targ(testForFail).Description("Run tests (fail-fast)")
	Tidy                 = targ.Targ(tidy).Description("Tidy go.mod")
	TodoCheck            = targ.Targ(todoCheck).Description("Check for TODOs")
	Watch                = targ.Targ(watch).Description("Watch and re-run checks")
)

type CoverageArgs struct {
	HTML bool `targ:"flag,desc=Open HTML report in browser"`
}

type coverageBlock struct {
	file       string
	startLine  int
	startCol   int
	endLine    int
	endCol     int
	statements int
	count      int
}

type deadFunc struct {
	name string
	line int
}

type lineAndCoverage struct {
	line     string
	coverage float64
}

type thinViolation struct {
	File   string
	Line   int
	Name   string
	Reason string
}

// analyzeThinness checks a file for non-thin declarations.
// Thin declarations include:
// - Type aliases (type X = pkg.X)
// - Constant re-exports (const X = pkg.X)
// - Interface definitions (just method signatures)
// - Functions with single return statement calling another package
// - Variables assigned from another package.
func analyzeThinness(path string) ([]thinViolation, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var violations []thinViolation

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if reason := checkFuncThinness(d); reason != "" {
				violations = append(violations, thinViolation{
					File:   path,
					Line:   fset.Position(d.Pos()).Line,
					Name:   funcDeclName(d),
					Reason: reason,
				})
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if v := checkSpecThinness(fset, path, d.Tok, spec); v != nil {
					violations = append(violations, *v)
				}
			}
		}
	}

	return violations, nil
}

func check(ctx context.Context) error {
	fmt.Println("Checking...")

	return targ.Deps(
		DeleteDeadcode, // no use doing anything else to dead code
		Fmt,            // after dead code removal, format code including imports
		Tidy,           // clean up the module dependencies
		Modernize,      // no use doing anything else to old code patterns
		CheckNils,      // is it nil free?
		CheckCoverage,  // does our code work?
		ReorderDecls,   // linter will yell about declaration order if not correct
		Lint,
		CheckThinAPI, // is public API thin wrappers only?
		targ.WithContext(ctx),
	)
}

func checkCoverage(ctx context.Context) error {
	fmt.Println("Checking coverage...")

	if err := targ.Deps(Test, targ.WithContext(ctx)); err != nil {
		return err
	}

	// Merge duplicate coverage blocks from cross-package testing
	if err := mergeCoverageBlocks("coverage.out"); err != nil {
		return fmt.Errorf("failed to merge coverage blocks: %w", err)
	}

	out, err := output(ctx, "go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return err
	}

	lines := strings.Split(out, "\n")
	linesAndCoverage := []lineAndCoverage{}

	for _, line := range lines {
		percentString := regexp.MustCompile(`\d+\.\d`).FindString(line)

		percent, err := strconv.ParseFloat(percentString, 64)
		if err != nil {
			return err
		}

		// Skip generated code, examples, and entry points
		if strings.Contains(line, "_string.go") ||
			strings.Contains(line, "generated_") ||
			strings.Contains(line, "/examples/") ||
			isEntryPointFile(line) {
			continue
		}

		// Skip summary line
		if strings.Contains(line, "total:") {
			continue
		}

		linesAndCoverage = append(linesAndCoverage, lineAndCoverage{line, percent})
	}

	slices.SortStableFunc(linesAndCoverage, func(a, b lineAndCoverage) int {
		if a.coverage < b.coverage {
			return -1
		}

		if a.coverage > b.coverage {
			return 1
		}

		return 0
	})
	lc := linesAndCoverage[0]

	sortedLines := make([]string, len(linesAndCoverage))
	for i := range linesAndCoverage {
		sortedLines[i] = linesAndCoverage[i].line
	}

	fmt.Println(strings.Join(sortedLines, "\n"))

	coverage := 80.0
	if lc.coverage < coverage {
		return fmt.Errorf("function coverage was less than the limit of %.1f:\n  %s", coverage, lc.line)
	}

	return nil
}

func checkCoverageForFail(ctx context.Context) error {
	fmt.Println("Checking coverage...")

	// Merge duplicate coverage blocks from cross-package testing
	if err := mergeCoverageBlocks("coverage.out"); err != nil {
		return fmt.Errorf("failed to merge coverage blocks: %w", err)
	}

	out, err := sh.OutputContext(ctx, "go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return err
	}

	lines := strings.Split(out, "\n")
	var minCoverage float64 = 100
	var minLine string

	for _, line := range lines {
		// Skip empty lines, summary, generated code, and entry points
		if line == "" ||
			strings.Contains(line, "_string.go") ||
			strings.Contains(line, "generated_") ||
			strings.Contains(line, "total:") ||
			isEntryPointFile(line) {
			continue
		}

		percentString := regexp.MustCompile(`\d+\.\d`).FindString(line)
		if percentString == "" {
			continue
		}

		percent, err := strconv.ParseFloat(percentString, 64)
		if err != nil {
			return err
		}

		if percent < minCoverage {
			minCoverage = percent
			minLine = line
		}
	}

	threshold := 80.0
	if minCoverage < threshold {
		return fmt.Errorf("function coverage was less than the limit of %.1f:\n  %s", threshold, minLine)
	}

	fmt.Printf("Coverage OK (min: %.1f%%)\n", minCoverage)

	return nil
}

func checkForFail(ctx context.Context) error {
	fmt.Println("Checking...")

	return targ.Deps(
		ReorderDeclsCheck,
		LintFast,
		LintForFail,
		Deadcode,
		CheckThinAPI,
		func() error { return targ.Deps(TestForFail, CheckCoverageForFail, targ.WithContext(ctx)) },
		CheckNilsForFail,
		targ.Parallel(),
		targ.WithContext(ctx),
	)
}

func checkNils(ctx context.Context) error {
	return targ.Deps(
		CheckNilsFix,
		CheckNilsForFail,
		targ.WithContext(ctx),
	)
}

func checkNilsFix(ctx context.Context) error {
	fmt.Println("Fixing nil issues...")
	return sh.RunContext(ctx, "nilaway", "-fix", "./...")
}

func checkNilsForFail(ctx context.Context) error {
	fmt.Println("Checking for nil issues...")
	return sh.RunContext(ctx, "nilaway", "./...")
}

func checkThinAPI(ctx context.Context) error {
	_ = ctx
	fmt.Println("Checking public API is thin wrappers...")

	// Find all non-internal, non-test Go files
	var files []string

	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories we don't care about
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "internal" || name == "testdata" {
				return filepath.SkipDir
			}

			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip files in internal directories (nested internal/)
		if strings.Contains(path, "/internal/") || strings.HasPrefix(path, "internal/") {
			return nil
		}

		// Skip examples (they demonstrate usage patterns)
		if strings.HasPrefix(path, "examples/") || strings.Contains(path, "/examples/") {
			return nil
		}

		// Skip generated files
		if strings.Contains(path, "generated_") {
			return nil
		}

		files = append(files, path)

		return nil
	})
	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	// Analyze each file
	var violations []thinViolation

	for _, file := range files {
		fileViolations, err := analyzeThinness(file)
		if err != nil {
			return fmt.Errorf("analyzing %s: %w", file, err)
		}

		violations = append(violations, fileViolations...)
	}

	if len(violations) == 0 {
		fmt.Printf("All %d public API files are thin wrappers.\n", len(files))

		return nil
	}

	// Group violations by file
	byFile := make(map[string][]thinViolation)
	for _, v := range violations {
		byFile[v.File] = append(byFile[v.File], v)
	}

	// Print violations grouped by file
	fmt.Printf("\nFound %d non-thin declarations in %d files:\n", len(violations), len(byFile))

	for file, fileViolations := range byFile {
		fmt.Printf("\n%s:\n", file)
		for _, v := range fileViolations {
			fmt.Printf("  %d: %s - %s\n", v.Line, v.Name, v.Reason)
		}
	}

	return fmt.Errorf("found %d non-thin declarations", len(violations))
}

func clean() {
	fmt.Println("Cleaning...")
	os.Remove("coverage.out")
}

func coverage(args CoverageArgs) error {
	if args.HTML {
		return sh.RunV("go", "tool", "cover", "-html=coverage.out")
	}
	return sh.RunV("go", "tool", "cover", "-func=coverage.out")
}

func deadcode(ctx context.Context) error {
	fmt.Println("Checking for dead code...")

	out, err := sh.OutputContext(ctx, "deadcode", "-test", "./...")
	if err != nil {
		return err
	}

	lines := strings.Split(out, "\n")
	filteredLines := []string{}

	for _, line := range lines {
		if line == "" {
			continue
		}

		filteredLines = append(filteredLines, line)
	}

	if len(filteredLines) > 0 {
		fmt.Println(strings.Join(filteredLines, "\n"))

		return errors.New("found dead code")
	}

	return nil
}

func deleteDeadFunctionsFromFile(filename string, funcs []deadFunc) (int, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return 0, fmt.Errorf("failed to parse file: %w", err)
	}

	toDelete := make(map[string]bool)
	for _, f := range funcs {
		toDelete[f.name] = true
	}

	newDecls := []ast.Decl{}
	deleted := 0

	for _, decl := range file.Decls {
		keep := true

		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			funcName := funcDecl.Name.Name

			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				recvType := funcDecl.Recv.List[0].Type
				var typeName string

				switch t := recvType.(type) {
				case *ast.StarExpr:
					if ident, ok := t.X.(*ast.Ident); ok {
						typeName = ident.Name
					}
				case *ast.Ident:
					typeName = t.Name
				}

				fullName := typeName + "." + funcName
				if toDelete[fullName] || toDelete[funcName] {
					keep = false
				}
			} else {
				if toDelete[funcName] {
					keep = false
				}
			}
		}

		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if toDelete[typeSpec.Name.Name] {
						keep = false
					}
				}
			}
		}

		if keep {
			newDecls = append(newDecls, decl)
		} else {
			deleted++
		}
	}

	if deleted == 0 {
		return 0, nil
	}

	file.Decls = newDecls

	var buf bytes.Buffer

	err = printer.Fprint(&buf, fset, file)
	if err != nil {
		return 0, fmt.Errorf("failed to print AST: %w", err)
	}

	err = os.WriteFile(filename, buf.Bytes(), 0o600)
	if err != nil {
		return 0, fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("  %s: deleted %d declarations\n", filename, deleted)

	return deleted, nil
}

func deleteDeadcode(ctx context.Context) error {
	fmt.Println("Deleting dead code...")

	out, err := output(ctx, "deadcode", "-test", "./...")
	if err != nil {
		return err
	}

	// Parse deadcode output: "file.go:123: unreachable func: FuncName"
	// Group by file
	fileToFuncs := make(map[string][]deadFunc)

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse: "impgen/run/codegen_interface.go:42: unreachable func: callStructData"
		parts := strings.Split(line, ": unreachable func: ")
		if len(parts) != 2 {
			continue
		}

		fileParts := strings.Split(parts[0], ":")
		if len(fileParts) < 2 {
			continue
		}

		file := fileParts[0]
		funcName := parts[1]

		// Skip generated files and test files
		if strings.Contains(file, "generated_") || strings.HasSuffix(file, ".qtpl.go") || strings.HasSuffix(file, "_test.go") {
			continue
		}

		lineNum, err := strconv.Atoi(fileParts[1])
		if err != nil {
			continue
		}

		fileToFuncs[file] = append(fileToFuncs[file], deadFunc{name: funcName, line: lineNum})
	}

	// Process each file
	totalDeleted := 0

	for file, funcs := range fileToFuncs {
		deleted, err := deleteDeadFunctionsFromFile(file, funcs)
		if err != nil {
			fmt.Printf("Warning: failed to process %s: %v\n", file, err)

			continue
		}

		totalDeleted += deleted
	}

	fmt.Printf("Deleted %d unreachable functions from %d files\n", totalDeleted, len(fileToFuncs))

	return nil
}

func findRedundantTests() error {
	config := testredundancy.Config{
		BaselineTests: []testredundancy.BaselineTestSpec{
			{Package: "./impgen/run", TestPattern: "TestUATConsistency"},
			{Package: "./UAT/core/...", TestPattern: ""},
			{Package: "./UAT/variations/...", TestPattern: ""},
		},
		CoverageThreshold: 80.0,
		PackageToAnalyze:  "./...",
		// Only measure coverage of impgen and imptest packages, not test fixtures
		CoveragePackages: "./impgen/...,./imptest/...",
	}

	return testredundancy.Find(config)
}

func fmtCode(ctx context.Context) error {
	fmt.Println("Formatting...")
	return sh.RunContext(ctx, "golangci-lint", "run", "-c", "dev/golangci-fmt.toml")
}

func fuzz() error {
	fmt.Println("Running fuzz tests...")

	// Find all test files
	testFiles, err := globs(".", []string{".go"})
	if err != nil {
		return fmt.Errorf("failed to find test files: %w", err)
	}

	// Filter to *_test.go files and find fuzz functions
	fuzzPattern := regexp.MustCompile(`^func (Fuzz\w+)\(`)
	type fuzzTest struct {
		dir  string
		name string
	}

	var fuzzTests []fuzzTest

	for _, file := range testFiles {
		if !strings.HasSuffix(file, "_test.go") {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			matches := fuzzPattern.FindStringSubmatch(line)
			if matches != nil {
				fuzzTests = append(fuzzTests, fuzzTest{
					dir:  "./" + filepath.Dir(file),
					name: matches[1],
				})
			}
		}
	}

	if len(fuzzTests) == 0 {
		fmt.Println("No fuzz tests found.")
		return nil
	}

	fmt.Printf("Found %d fuzz tests.\n", len(fuzzTests))

	// Run each fuzz test
	for _, test := range fuzzTests {
		fmt.Printf("  Fuzzing %s in %s...\n", test.name, test.dir)

		err := sh.Run("go", "test", test.dir, "-fuzz=^"+test.name+"$", "-fuzztime=1000x")
		if err != nil {
			return fmt.Errorf("fuzz test %s failed: %w", test.name, err)
		}
	}

	fmt.Println("All fuzz tests passed.")

	return nil
}

func generate() error {
	fmt.Println("Generating...")

	// Run go generate with modified PATH
	cmd := exec.Command("go", "generate", "./...")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func globs(dir string, ext []string) ([]string, error) {
	files := []string{}

	err := filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("unable to find all glob matches: %w", err)
		}

		for _, each := range ext {
			if filepath.Ext(path) == each {
				files = append(files, path)

				return nil
			}
		}

		return nil
	})

	return files, err
}

// hasRelevantChanges returns true if the changeset contains files we care about.
// Filters out generated files and build artifacts that Check() itself creates.
func hasRelevantChanges(changes file.ChangeSet) bool {
	allFiles := append(append(changes.Added, changes.Removed...), changes.Modified...)

	for _, f := range allFiles {
		// Skip generated test files
		if strings.Contains(f, "generated_") {
			continue
		}
		// Skip coverage output
		if strings.HasSuffix(f, "coverage.out") {
			continue
		}
		// Found a relevant change
		return true
	}

	return false
}

func installTools() error {
	fmt.Println("Installing development tools...")
	return sh.Run("./dev/dev-install.sh")
}

// Helper Functions

// checkFuncThinness returns a reason string if the function is not thin, empty string if thin.
func checkFuncThinness(fn *ast.FuncDecl) string {
	if fn.Body == nil {
		return "" // Interface method or external function
	}

	stmts := fn.Body.List

	// Empty function is thin
	if len(stmts) == 0 {
		return ""
	}

	// Single statement is potentially thin
	if len(stmts) == 1 {
		// Single return statement
		if ret, ok := stmts[0].(*ast.ReturnStmt); ok {
			return checkReturnThinness(ret)
		}
		// Single expression statement (e.g., targ.Register(...) or pkg.Func())
		if expr, ok := stmts[0].(*ast.ExprStmt); ok {
			if isExternalCall(expr.X) {
				return ""
			}
		}
	}

	// Check for simple error-handling wrapper pattern:
	// result, err := pkg.Func(...)
	// if err != nil { return ... }
	// return result
	if len(stmts) >= 2 && len(stmts) <= 3 {
		if isSimpleErrorWrapper(stmts) {
			return ""
		}
	}

	return fmt.Sprintf("has %d statements (thin functions have 1 or simple error handling)", len(stmts))
}

// checkReturnThinness checks if a return statement is a thin wrapper.
func checkReturnThinness(ret *ast.ReturnStmt) string {
	if len(ret.Results) == 0 {
		return "" // Empty return is thin
	}

	// Single result
	if len(ret.Results) == 1 {
		result := ret.Results[0]

		// Call to another package
		if call, ok := result.(*ast.CallExpr); ok {
			if isExternalCall(call) {
				return ""
			}

			return "calls local function, not external package"
		}
		// Returning a variable or selector (like pkg.Var)
		if isExternalSelector(result) {
			return ""
		}
		// Returning a literal
		if isBasicLit(result) {
			return ""
		}
		// Returning an identifier (variable, nil, true, false)
		if _, ok := result.(*ast.Ident); ok {
			return ""
		}
	}

	// Multiple results - check if it's a call with multiple returns
	// e.g., return pkg.Func(args...)
	if len(ret.Results) >= 1 {
		if call, ok := ret.Results[0].(*ast.CallExpr); ok {
			if isExternalCall(call) {
				return ""
			}
		}
	}

	return "return expression is not a simple external call or re-export"
}

// checkSpecThinness checks if a type/const/var spec is thin.
func checkSpecThinness(fset *token.FileSet, path string, tok token.Token, spec ast.Spec) *thinViolation {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return checkTypeSpecThinness(fset, path, s)
	case *ast.ValueSpec:
		return checkValueSpecThinness(fset, path, tok, s)
	}

	return nil
}

// checkTypeSpecThinness checks if a type declaration is thin.
func checkTypeSpecThinness(fset *token.FileSet, path string, ts *ast.TypeSpec) *thinViolation {
	// Type alias is thin: type X = pkg.Y
	if ts.Assign.IsValid() {
		return nil
	}

	// Interface definitions are thin (just signatures)
	if _, ok := ts.Type.(*ast.InterfaceType); ok {
		return nil
	}

	// Struct with fields is not thin
	if st, ok := ts.Type.(*ast.StructType); ok {
		if st.Fields != nil && len(st.Fields.List) > 0 {
			return &thinViolation{
				File:   path,
				Line:   fset.Position(ts.Pos()).Line,
				Name:   "type " + ts.Name.Name,
				Reason: "struct with fields (should be in internal/)",
			}
		}
		// Empty struct is thin
		return nil
	}

	// Other type definitions (not aliases) are not thin
	return &thinViolation{
		File:   path,
		Line:   fset.Position(ts.Pos()).Line,
		Name:   "type " + ts.Name.Name,
		Reason: "type definition (not alias) should be in internal/",
	}
}

// checkValueSpecThinness checks if a const/var declaration is thin.
func checkValueSpecThinness(fset *token.FileSet, path string, tok token.Token, vs *ast.ValueSpec) *thinViolation {
	// Constants that reference external package are thin
	if tok == token.CONST {
		for _, val := range vs.Values {
			if !isExternalSelector(val) && !isBasicLit(val) {
				return &thinViolation{
					File:   path,
					Line:   fset.Position(vs.Pos()).Line,
					Name:   "const " + nameListString(vs.Names),
					Reason: "const value is not a re-export or literal",
				}
			}
		}

		return nil
	}

	// Variables: check if assigned from external package or is a function call
	if tok == token.VAR {
		for _, val := range vs.Values {
			if isExternalSelector(val) || isExternalCall(val) || isBasicLit(val) {
				continue
			}
			// Allow nil assignments
			if ident, ok := val.(*ast.Ident); ok && ident.Name == "nil" {
				continue
			}

			return &thinViolation{
				File:   path,
				Line:   fset.Position(vs.Pos()).Line,
				Name:   "var " + nameListString(vs.Names),
				Reason: "var value is not a re-export, external call, or literal",
			}
		}
	}

	return nil
}

func funcDeclName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recvType := fn.Recv.List[0].Type
		var typeName string

		switch t := recvType.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				typeName = "*" + ident.Name
			}
		case *ast.Ident:
			typeName = t.Name
		}

		return fmt.Sprintf("(%s).%s", typeName, fn.Name.Name)
	}

	return fn.Name.Name
}

func isBasicLit(expr ast.Expr) bool {
	_, ok := expr.(*ast.BasicLit)

	return ok
}

// isExternalCall checks if a call expression calls an external package.
func isExternalCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Check if the function being called is pkg.Func
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		// pkg.Func() - sel.X is the package identifier
		if _, ok := sel.X.(*ast.Ident); ok {
			return true
		}
	}

	return false
}

// isExternalSelector checks if an expression is pkg.Something.
func isExternalSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// pkg.Something
	_, ok = sel.X.(*ast.Ident)

	return ok
}

// isSimpleErrorWrapper checks for pattern:
//
//	result, err := pkg.Func(...)
//	if err != nil { return ... }
//	return result
func isSimpleErrorWrapper(stmts []ast.Stmt) bool {
	// First statement should be assignment
	assign, ok := stmts[0].(*ast.AssignStmt)
	if !ok {
		return false
	}

	// Should be := or =
	if assign.Tok != token.DEFINE && assign.Tok != token.ASSIGN {
		return false
	}

	// RHS should be external call
	if len(assign.Rhs) != 1 {
		return false
	}

	if !isExternalCall(assign.Rhs[0]) {
		return false
	}

	// Second statement should be if err != nil
	ifStmt, ok := stmts[1].(*ast.IfStmt)
	if !ok {
		return false
	}

	// Check it's checking err != nil
	bin, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}

	// If there's a third statement, it should be a return
	if len(stmts) == 3 {
		_, ok := stmts[2].(*ast.ReturnStmt)

		return ok
	}

	return true
}

func nameListString(names []*ast.Ident) string {
	var result []string
	for _, n := range names {
		result = append(result, n.Name)
	}

	return strings.Join(result, ", ")
}

// isEntryPointCoverageLine checks coverage.out format lines (e.g., "module/file.go:1.1,2.2 1 0")
func isEntryPointCoverageLine(line string) bool {
	// main.go files are CLI entry points
	if strings.Contains(line, "/main.go:") {
		return true
	}

	// Top-level module files: github.com/toejough/targ/<file>.go
	const modulePrefix = "github.com/toejough/targ/"

	idx := strings.Index(line, modulePrefix)
	if idx == -1 {
		return false
	}

	afterModule := line[idx+len(modulePrefix):]

	// Find where the file path ends (at the colon)
	colonIdx := strings.Index(afterModule, ":")
	if colonIdx == -1 {
		return false
	}

	pathPart := afterModule[:colonIdx]

	return !strings.Contains(pathPart, "/")
}

// isEntryPointFile returns true for files excluded from coverage checks:
// - main.go files (thin CLI entry points)
// - Top-level module files (re-exports and dependency injection wrappers)
// These files should only contain transparent re-exports or thin wrappers
// that inject dependencies into internal APIs.
// This works on "go tool cover -func" output format.
func isEntryPointFile(coverageLine string) bool {
	return isEntryPointCoverageLine(coverageLine)
}

func isGeneratedFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	buf := make([]byte, 200)

	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read %s: %w", path, err)
	}

	content := string(buf[:n])

	return strings.Contains(content, "Code generated") || strings.Contains(content, "DO NOT EDIT"), nil
}

func lint(ctx context.Context) error {
	fmt.Println("Linting...")
	return sh.RunContext(ctx, "golangci-lint", "run", "-c", "dev/golangci-lint.toml")
}

func lintFast(ctx context.Context) error {
	fmt.Println("Running fast linters...")

	return sh.RunContext(ctx,
		"golangci-lint", "run",
		"-c", "dev/golangci-fast.toml",
		"--allow-parallel-runners",
	)
}

func lintForFail(ctx context.Context) error {
	fmt.Println("Linting to check for overall pass/fail...")

	return sh.RunContext(ctx,
		"golangci-lint", "run",
		"-c", "dev/golangci-lint.toml",
		"--fix=false",
		"--max-issues-per-linter=1",
		"--max-same-issues=1",
		"--allow-parallel-runners",
	)
}

// mergeCoverageBlocks merges duplicate coverage blocks in a coverage file.
// This handles the case where multiple test packages cover the same code.
func mergeCoverageBlocks(coverageFile string) error {
	data, err := os.ReadFile(coverageFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil
	}

	// First line is mode
	modeLine := lines[0]

	// Merge blocks by key (file:start,end statements)
	blocks := make(map[string]coverageBlock)

	for _, line := range lines[1:] {
		if line == "" {
			continue
		}

		block, err := parseCoverageBlock(line)
		if err != nil {
			continue
		}

		key := fmt.Sprintf("%s:%d.%d,%d.%d %d",
			block.file, block.startLine, block.startCol,
			block.endLine, block.endCol, block.statements)

		if existing, ok := blocks[key]; ok {
			existing.count += block.count
			blocks[key] = existing
		} else {
			blocks[key] = block
		}
	}

	// Write merged blocks
	var result strings.Builder

	result.WriteString(modeLine)
	result.WriteString("\n")

	for _, block := range blocks {
		fmt.Fprintf(&result, "%s:%d.%d,%d.%d %d %d\n",
			block.file, block.startLine, block.startCol,
			block.endLine, block.endCol, block.statements, block.count)
	}

	return os.WriteFile(coverageFile, []byte(result.String()), 0o600)
}

func modernize(ctx context.Context) error {
	fmt.Println("Modernizing codebase...")

	return sh.RunContext(ctx, "go", "run", "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest",
		"-fix", "./...")
}

func mutate() error {
	fmt.Println("Running mutation tests...")

	if err := targ.Deps(CheckForFail); err != nil {
		return err
	}

	return sh.Run(
		"go",
		"test",
		"-timeout=0",
		"-tags=mutation",
		"-v",
		"./dev/...",
		"-run=TestMutation",
	)
}

// output runs a command and captures stdout only (stderr goes to os.Stderr).
func output(ctx context.Context, command string, args ...string) (string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	return strings.TrimSuffix(buf.String(), "\n"), err
}

func parseBlockID(blockID string) (file string, startLine, startCol, endLine, endCol int, err error) {
	fileParts := strings.Split(blockID, ":")
	if len(fileParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid block ID format: %s", blockID)
	}

	file = fileParts[0]

	rangeParts := strings.Split(fileParts[1], ",")
	if len(rangeParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid range format: %s", blockID)
	}

	startParts := strings.Split(rangeParts[0], ".")
	if len(startParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid start position: %s", blockID)
	}

	endParts := strings.Split(rangeParts[1], ".")
	if len(endParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid end position: %s", blockID)
	}

	startLine, _ = strconv.Atoi(startParts[0])
	startCol, _ = strconv.Atoi(startParts[1])
	endLine, _ = strconv.Atoi(endParts[0])
	endCol, _ = strconv.Atoi(endParts[1])

	return file, startLine, startCol, endLine, endCol, nil
}

func parseCoverageBlock(line string) (coverageBlock, error) {
	// Format: file:startLine.startCol,endLine.endCol statements count
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return coverageBlock{}, fmt.Errorf("invalid line format")
	}

	blockID := parts[0]
	statements, _ := strconv.Atoi(parts[1])
	count, _ := strconv.Atoi(parts[2])

	file, startLine, startCol, endLine, endCol, err := parseBlockID(blockID)
	if err != nil {
		return coverageBlock{}, err
	}

	return coverageBlock{
		file:       file,
		startLine:  startLine,
		startCol:   startCol,
		endLine:    endLine,
		endCol:     endCol,
		statements: statements,
		count:      count,
	}, nil
}

func reorderDecls(ctx context.Context) error {
	_ = ctx // Reserved for future cancellation support
	fmt.Println("Reordering declarations...")

	files, err := globs(".", []string{".go"})
	if err != nil {
		return fmt.Errorf("failed to find Go files: %w", err)
	}

	reorderedCount := 0

	for _, file := range files {
		// Skip generated files by name pattern
		if strings.Contains(file, "generated_") {
			continue
		}
		// Skip vendor
		if strings.HasPrefix(file, "vendor/") {
			continue
		}
		// Skip hidden directories
		if strings.Contains(file, "/.") {
			continue
		}

		// Skip files with generated markers (e.g., .qtpl.go files)
		isGenerated, err := isGeneratedFile(file)
		if err != nil {
			return fmt.Errorf("failed to check if %s is generated: %w", file, err)
		}

		if isGenerated {
			continue
		}

		// Read file
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		// Reorder
		reordered, err := reorder.Source(string(content))
		if err != nil {
			fmt.Printf("Warning: failed to reorder %s: %v\n", file, err)

			continue
		}

		// Write back if changed
		if string(content) != reordered {
			err = os.WriteFile(file, []byte(reordered), 0o600)
			if err != nil {
				return fmt.Errorf("failed to write %s: %w", file, err)
			}

			fmt.Printf("  Reordered: %s\n", file)
			reorderedCount++
		}
	}

	fmt.Printf("Reordered %d file(s).\n", reorderedCount)

	return nil
}

func reorderDeclsCheck(ctx context.Context) error {
	_ = ctx // Reserved for future cancellation support
	fmt.Println("Checking declaration order...")

	files, err := globs(".", []string{".go"})
	if err != nil {
		return fmt.Errorf("failed to find Go files: %w", err)
	}

	outOfOrderFiles := 0
	filesProcessed := 0

	for _, file := range files {
		// Skip generated files by name pattern
		if strings.Contains(file, "generated_") {
			continue
		}
		// Skip vendor
		if strings.HasPrefix(file, "vendor/") {
			continue
		}
		// Skip hidden directories
		if strings.Contains(file, "/.") {
			continue
		}

		// Skip files with generated markers (e.g., .qtpl.go files)
		isGenerated, err := isGeneratedFile(file)
		if err != nil {
			return fmt.Errorf("failed to check if %s is generated: %w", file, err)
		}

		if isGenerated {
			continue
		}

		// Read file
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		// Analyze section order
		sectionOrder, err := reorder.AnalyzeSectionOrder(string(content))
		if err != nil {
			fmt.Printf("Warning: failed to analyze %s: %v\n", file, err)

			continue
		}

		filesProcessed++

		// Get reordered version
		reordered, err := reorder.Source(string(content))
		if err != nil {
			fmt.Printf("Warning: failed to reorder %s: %v\n", file, err)

			continue
		}

		// Check if reordering would change the file
		if string(content) != reordered {
			outOfOrderFiles++
			fmt.Printf("\n%s:\n", file)

			// Print section analysis
			fmt.Println("  Current order:")

			for i, section := range sectionOrder.Sections {
				posStr := fmt.Sprintf("%d", i+1)
				expectedNote := ""

				if section.Expected != i+1 {
					expectedNote = fmt.Sprintf(" <- should be #%d", section.Expected)
				}

				fmt.Printf("    %s. %-24s%s\n", posStr, section.Name, expectedNote)
			}

			// Identify sections that are out of place
			outOfPlace := []string{}

			for i, section := range sectionOrder.Sections {
				if section.Expected != i+1 {
					outOfPlace = append(outOfPlace, fmt.Sprintf("%s (at #%d, should be #%d)",
						section.Name, i+1, section.Expected))
				}
			}

			if len(outOfPlace) > 0 {
				fmt.Printf("  Sections out of place: %s\n", strings.Join(outOfPlace, ", "))
			}

			// Show diff
			diff := textdiff.Unified(file+" (current)", file+" (reordered)", string(content), reordered)
			if diff != "" {
				fmt.Printf("\n%s\n", diff)
			}
		}
	}

	if outOfOrderFiles > 0 {
		fmt.Printf("\n%d file(s) need reordering (out of %d processed). Run 'targ reorder-decls' to fix.\n", outOfOrderFiles, filesProcessed)

		return fmt.Errorf("%d file(s) need reordering", outOfOrderFiles)
	}

	fmt.Printf("All files are correctly ordered (%d files processed).\n", filesProcessed)

	return nil
}

func test(ctx context.Context) error {
	fmt.Println("Running unit tests...")

	if err := targ.Deps(Generate); err != nil {
		return err
	}

	// Use -count=1 to disable caching so coverage is regenerated
	err := sh.RunContext(ctx,
		"go",
		"test",
		"-timeout=2m",
		"-race",
		"-count=1",
		"-coverprofile=coverage.out",
		"-coverpkg=./...",
		"-cover",
		"./...",
	)
	if err != nil {
		return err
	}

	// Strip coverage lines for generated templates and entry point files
	// Entry points (main.go, top-level module files) should only contain
	// re-exports and dependency injection - no testable logic.
	data, err := os.ReadFile("coverage.out")
	if err != nil {
		return fmt.Errorf("failed to read coverage.out: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var filtered []string

	for _, line := range lines {
		if strings.Contains(line, ".qtpl:") ||
			isEntryPointCoverageLine(line) {
			continue
		}

		filtered = append(filtered, line)
	}

	err = os.WriteFile("coverage.out", []byte(strings.Join(filtered, "\n")), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write coverage.out: %w", err)
	}

	return nil
}

func testForFail(ctx context.Context) error {
	fmt.Println("Running unit tests for overall pass/fail...")

	if err := targ.Deps(Generate); err != nil {
		return err
	}

	return sh.RunContext(ctx,
		"go",
		"test",
		"-buildvcs=false",
		"-timeout=30s",
		"-coverprofile=coverage.out",
		"-coverpkg=./impgen/...,./imptest/...",
		"./...",
		"-failfast",
	)
}

func tidy(ctx context.Context) error {
	fmt.Println("Tidying go.mod...")
	return sh.RunContext(ctx, "go", "mod", "tidy")
}

func todoCheck() error {
	fmt.Println("Checking for TODOs...")
	return sh.Run("golangci-lint", "run", "-c", "dev/golangci-todos.toml")
}

func watch(ctx context.Context) error {
	fmt.Println("Watching...")

	var (
		cancelCheck context.CancelFunc
		checkMu     sync.Mutex
	)

	return file.Watch(ctx, []string{"**/*.go", "**/*.fish", "**/*.toml"}, file.WatchOptions{}, func(changes file.ChangeSet) error {
		// Filter out generated files and coverage output to avoid infinite loops
		if !hasRelevantChanges(changes) {
			return nil
		}

		// Log the changed files for debugging
		fmt.Println("Change detected in:")
		for _, f := range changes.Added {
			fmt.Printf("  + %s\n", f)
		}
		for _, f := range changes.Modified {
			fmt.Printf("  ~ %s\n", f)
		}
		for _, f := range changes.Removed {
			fmt.Printf("  - %s\n", f)
		}

		checkMu.Lock()
		defer checkMu.Unlock()

		// Cancel any running check
		if cancelCheck != nil {
			fmt.Println("Cancelling previous check...")
			cancelCheck()
		}

		// Create new cancellable context for this check
		checkCtx, cancel := context.WithCancel(ctx)
		cancelCheck = cancel

		targ.ResetDeps() // Clear execution cache so targets run again

		err := check(checkCtx)
		if errors.Is(err, context.Canceled) {
			fmt.Println("Check cancelled, restarting...")
		} else if err != nil {
			fmt.Println("continuing to watch after check failure (see errors above)")
		} else {
			fmt.Println("continuing to watch after all checks passed!")
		}

		return nil // Don't stop watching on error
	})
}
