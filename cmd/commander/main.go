package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"text/template"
)

func main() {
	// 1. Scan current directory for go files
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		fmt.Printf("Error parsing directory: %v\n", err)
		os.Exit(1)
	}

	mainPkg, ok := pkgs["main"]
	if !ok {
		fmt.Println("No package main found in current directory")
		os.Exit(1)
	}

	// 2. Find exported structs
	var structs []string
	for _, file := range mainPkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			// Look for TypeSpec
			if t, ok := n.(*ast.TypeSpec); ok {
				// Check if it's a struct
				if _, ok := t.Type.(*ast.StructType); ok {
					// Check if exported
					if t.Name.IsExported() {
						structs = append(structs, t.Name.Name)
					}
				}
			}
			return true
		})
	}

	if len(structs) == 0 {
		fmt.Println("No exported structs found")
		os.Exit(1)
	}

	// 3. Generate bootstrap file
	tmpl := `
package main

import (
	"commander"
)

func main() {
	// Auto-detected commands
	cmds := []interface{}{
{{- range .Structs }}
		&{{ . }}{},
{{- end }}
	}
	
	// Filter roots
	roots := commander.DetectRootCommands(cmds...)
	
	// Run
	commander.Run(roots...)
}
`
	t := template.Must(template.New("main").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, struct{ Structs []string }{Structs: structs}); err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		os.Exit(1)
	}

	filename := "commander_bootstrap.go"
	if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		fmt.Printf("Error writing bootstrap file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(filename)

	// 4. Run "go run ."
	// We need to pass arguments through
	args := []string{"run", "."}
	args = append(args, os.Args[1:]...)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		// Don't print error if it's just exit code
		os.Exit(1)
	}
}
