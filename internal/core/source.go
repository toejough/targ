package core

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

var (
	errCallerFailed = errors.New("runtime.Caller failed")
	errFuncForPCNil = errors.New("runtime.FuncForPC returned nil")
)

// callerPackagePath returns the package path of the caller at the given stack depth.
// depth 0 = callerPackagePath itself, 1 = direct caller, 2 = caller's caller, etc.
func callerPackagePath(depth int) (string, error) {
	// Get program counter at the specified depth
	// We add 1 to depth because depth 0 would be this function itself
	pc, _, _, ok := runtime.Caller(depth + 1)
	if !ok {
		return "", fmt.Errorf("%w at depth %d", errCallerFailed, depth)
	}

	// Get function info from program counter
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "", fmt.Errorf("%w for depth %d", errFuncForPCNil, depth)
	}

	// Extract package path from fully qualified function name
	return extractPackagePath(fn.Name()), nil
}

// extractPackagePath parses package path from fully qualified function name.
// It handles various runtime.FuncForPC formats:
// - "github.com/user/repo/pkg.Func" -> "github.com/user/repo/pkg"
// - "github.com/user/repo.init" -> "github.com/user/repo"
// - "github.com/user/repo.init.0" -> "github.com/user/repo"
// - "github.com/user/repo.init.func1" -> "github.com/user/repo"
func extractPackagePath(funcName string) string {
	if funcName == "" {
		return ""
	}

	// Find the last dot that separates package from function/method name
	// We need to be careful because:
	// 1. Package paths contain dots (e.g., "github.com")
	// 2. Function names can contain dots (e.g., "init.0", "init.func1")
	// 3. We want everything before the last segment after the final slash

	// Strategy: Find the last slash, then find the first dot after it
	lastSlash := strings.LastIndex(funcName, "/")
	if lastSlash == -1 {
		// No slashes - this is likely just "package.Func"
		// Find first dot
		before, _, ok := strings.Cut(funcName, ".")
		if !ok {
			return funcName
		}

		return before
	}

	// Find the first dot after the last slash
	afterSlash := funcName[lastSlash+1:]

	dotIdx := strings.Index(afterSlash, ".")
	if dotIdx == -1 {
		// No dot after last slash - return the whole thing
		return funcName
	}

	// Return everything up to (but not including) this dot
	return funcName[:lastSlash+1+dotIdx]
}
