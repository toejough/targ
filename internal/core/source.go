package core

import "strings"

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
		dotIdx := strings.Index(funcName, ".")
		if dotIdx == -1 {
			return funcName
		}
		return funcName[:dotIdx]
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
