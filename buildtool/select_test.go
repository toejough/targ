package buildtool

import (
	"io/fs"
	"strings"
	"testing"
)

func TestSelectTaggedDirs_DepthGatingErrorsOnMultipleDirs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root"}
	done := make(chan struct{})
	var err error

	go func() {
		_, err = SelectTaggedDirs(fsMock.Interface(), opts)
		close(done)
	}()

	fsMock.ReadDir.ExpectCalledWithExactly("/root").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "pkg1", dir: true},
		fakeDirEntry{name: "pkg2", dir: true},
	}, nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg1").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build commander

package pkg1

func Hello() {}
`), nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build commander

package pkg2

func Hi() {}
`), nil)

	<-done

	if err == nil {
		t.Fatal("expected error for multiple tagged dirs at same depth")
	}
	if !strings.Contains(err.Error(), "/root/pkg1") || !strings.Contains(err.Error(), "/root/pkg2") {
		t.Fatalf("expected error to list conflicting paths, got: %v", err)
	}
}

func TestSelectTaggedDirs_MultiPackageReturnsAllDirs(t *testing.T) {
	fsMock := MockFileSystem(t)
	opts := Options{StartDir: "/root", MultiPackage: true}
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
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg1/cmd.go").InjectReturnValues([]byte(`//go:build commander

package pkg1
`), nil)
	fsMock.ReadDir.ExpectCalledWithExactly("/root/pkg2").InjectReturnValues([]fs.DirEntry{
		fakeDirEntry{name: "cmd.go", dir: false},
	}, nil)
	fsMock.ReadFile.ExpectCalledWithExactly("/root/pkg2/cmd.go").InjectReturnValues([]byte(`//go:build commander

package pkg2
`), nil)

	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("expected 2 tagged dirs, got %d", len(dirs))
	}
}
