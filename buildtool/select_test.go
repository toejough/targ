package buildtool

import (
	"io/fs"
	"testing"
)

func TestSelectTaggedDirs_ReturnsAllDirs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var (
		dirs []TaggedDir
		err  error
	)

	go func() {
		dirs, err = SelectTaggedDirs(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "pkg1", dir: true},
		fakeDirEntry{name: "pkg2", dir: true},
	}, nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg1").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg1

func Hello() {}
`), nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build targ

package pkg2

func Hi() {}
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("expected 2 tagged dirs, got %d", len(dirs))
	}
}
