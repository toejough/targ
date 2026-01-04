package targ

import (
	// "fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"sync"
)

var (
	fileCache = make(map[string]*ast.File)
	fset      = token.NewFileSet()
	cacheLock sync.Mutex
)

// getMethodDoc extracts the documentation comment for a method.
func getMethodDoc(method reflect.Method) string {
	pc := method.Func.Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}
	file, _ := fn.FileLine(pc)

	// Skip if not a user file (e.g. stdlib or generated wrapper)
	// Simple check: if file doesn't exist or we can't read it

	f, err := getParsedFile(file)
	if err != nil {
		// fmt.Printf("Debug: failed to parse file %s: %v\n", file, err)
		return ""
	}

	var doc string

	// Search for the function declaration matching this method
	// We need to match Method Name and Receiver Type Name
	methodName := method.Name
	// Receiver type
	recvType := method.Type.In(0)
	if recvType.Kind() == reflect.Ptr {
		recvType = recvType.Elem()
	}
	recvName := recvType.Name()

	ast.Inspect(f, func(n ast.Node) bool {
		if fnDecl, ok := n.(*ast.FuncDecl); ok {
			if fnDecl.Name.Name == methodName {
				// Check receiver
				if fnDecl.Recv != nil && len(fnDecl.Recv.List) > 0 {
					typeExpr := fnDecl.Recv.List[0].Type
					// Handle pointer receiver in AST (*Type)
					if star, ok := typeExpr.(*ast.StarExpr); ok {
						if ident, ok := star.X.(*ast.Ident); ok {
							if ident.Name == recvName {
								doc = fnDecl.Doc.Text()
								return false // Found
							}
						}
					} else if ident, ok := typeExpr.(*ast.Ident); ok {
						// Handle value receiver (Type)
						if ident.Name == recvName {
							doc = fnDecl.Doc.Text()
							return false // Found
						}
					}
				}
			}
		}
		return true
	})

	return doc
}

func getParsedFile(path string) (*ast.File, error) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if f, ok := fileCache[path]; ok {
		return f, nil
	}

	// Mode: ParseComments is essential
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	fileCache[path] = f
	return f, nil
}
