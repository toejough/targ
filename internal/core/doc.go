package core

import (
	// "fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"sync"
)

// unexported variables.
var (
	cacheLock sync.Mutex
	fileCache = make(map[string]*ast.File)
	fset      = token.NewFileSet()
)

// getMethodDoc extracts the documentation comment for a method.
func getMethodDoc(method reflect.Method) string {
	file := methodSourceFile(method)
	if file == "" {
		return ""
	}

	f, err := getParsedFile(file)
	if err != nil {
		return ""
	}

	recvName := receiverTypeName(method)

	return findMethodDocInAST(f, method.Name, recvName)
}

// methodSourceFile returns the source file path for a method.
func methodSourceFile(method reflect.Method) string {
	pc := method.Func.Pointer()

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}

	file, _ := fn.FileLine(pc)

	return file
}

// receiverTypeName extracts the receiver type name from a method.
func receiverTypeName(method reflect.Method) string {
	recvType := method.Type.In(0)
	if recvType.Kind() == reflect.Ptr {
		recvType = recvType.Elem()
	}

	return recvType.Name()
}

// findMethodDocInAST searches the AST for a method's documentation.
func findMethodDocInAST(f *ast.File, methodName, recvName string) string {
	var doc string

	ast.Inspect(f, func(n ast.Node) bool {
		fnDecl, ok := n.(*ast.FuncDecl)
		if !ok || fnDecl.Name.Name != methodName {
			return true
		}

		if matchesReceiver(fnDecl, recvName) {
			doc = fnDecl.Doc.Text()
			return false
		}

		return true
	})

	return doc
}

// matchesReceiver checks if a function declaration has the expected receiver type.
func matchesReceiver(fnDecl *ast.FuncDecl, recvName string) bool {
	if fnDecl.Recv == nil || len(fnDecl.Recv.List) == 0 {
		return false
	}

	typeExpr := fnDecl.Recv.List[0].Type

	return extractReceiverName(typeExpr) == recvName
}

// extractReceiverName extracts the type name from a receiver expression.
func extractReceiverName(typeExpr ast.Expr) string {
	if star, ok := typeExpr.(*ast.StarExpr); ok {
		if ident, ok := star.X.(*ast.Ident); ok {
			return ident.Name
		}
	}

	if ident, ok := typeExpr.(*ast.Ident); ok {
		return ident.Name
	}

	return ""
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
