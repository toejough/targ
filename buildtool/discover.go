// Package buildtool provides utilities for discovering targ packages.
package buildtool

import (
	"github.com/toejough/targ/internal/discover"
)

// Exported variables.
var (
	ErrMainFunctionNotAllowed = discover.ErrMainFunctionNotAllowed
	ErrMultiplePackageNames   = discover.ErrMultiplePackageNames
	ErrNoTaggedFiles          = discover.ErrNoTaggedFiles
)

// FileInfo holds path information for a discovered file.
type FileInfo = discover.FileInfo

// FileSystem abstracts file operations for testing.
type FileSystem = discover.FileSystem

// Options configures the discovery process.
type Options = discover.Options

// PackageInfo holds metadata about a discovered targ package.
type PackageInfo = discover.PackageInfo

// TaggedDir represents a directory containing targ-tagged files.
type TaggedDir = discover.TaggedDir

// TaggedFile holds a file path and its contents.
type TaggedFile = discover.TaggedFile

// Thin wrapper functions.

// Discover finds all packages with targ-tagged files and parses their info.
func Discover(filesystem FileSystem, opts Options) ([]PackageInfo, error) {
	return discover.Discover(filesystem, opts)
}

// SelectTaggedDirs returns directories containing targ-tagged files.
func SelectTaggedDirs(filesystem FileSystem, opts Options) ([]TaggedDir, error) {
	return discover.SelectTaggedDirs(filesystem, opts)
}

// TaggedFiles returns all files with the specified build tag.
func TaggedFiles(filesystem FileSystem, opts Options) ([]TaggedFile, error) {
	return discover.TaggedFiles(filesystem, opts)
}
