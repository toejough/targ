package buildtool_test

import (
	"testing"

	"github.com/toejough/targ/buildtool"
)

func TestSelectTaggedDirs_ReturnsAllDirs(t *testing.T) {
	testMultipleTaggedPackages(t, func(fsMock *FileSystemMockHandle, opts buildtool.Options) (int, error) {
		dirs, err := buildtool.SelectTaggedDirs(fsMock.Mock, opts)
		return len(dirs), err
	})
}
