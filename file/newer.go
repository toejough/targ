package file

import (
	internal "github.com/toejough/targ/internal/file"
)

// Newer reports whether inputs are newer than outputs, or when outputs are empty,
// whether the input match set or file modtimes changed since the last run.
func Newer(inputs, outputs []string) (bool, error) {
	return internal.Newer(inputs, outputs, func(patterns []string) ([]string, error) {
		return Match(patterns...)
	}, nil, nil)
}
