package buildtool

import (
	"testing"
)

func TestSelectTaggedDirs_ReturnsAllDirs(t *testing.T) {
	testMultipleTaggedPackages(t, func(fsMock *FileSystemMockHandle, opts Options) (int, error) {
		dirs, err := SelectTaggedDirs(fsMock.Mock, opts)
		return len(dirs), err
	})
}
