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

type FileInfo = discover.FileInfo

type FileSystem = discover.FileSystem

type Options = discover.Options

type PackageInfo = discover.PackageInfo

type TaggedDir = discover.TaggedDir

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
