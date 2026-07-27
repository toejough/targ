//go:build integration

package runner_test

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toejough/targ/internal/runner"
)

// TestIntegrationEnsureTargDependencyKeepsThePin is the regression test for #41.
// It must FAIL against the pre-fix unversioned `go get`.
func TestIntegrationEnsureTargDependencyKeepsThePin(t *testing.T) {
	newStandInProxy(t)

	t.Run("existing require line is left byte-identical", func(t *testing.T) {
		root := newConsumerModule(t, "v1.0.0")

		before, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			t.Fatalf("reading go.mod: %v", err)
		}

		var errOut bytes.Buffer

		runner.ExportEnsureTargDependency(runner.TargDependency{ModulePath: standInModule}, root, &errOut)

		after, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			t.Fatalf("reading go.mod: %v", err)
		}

		if !bytes.Equal(before, after) {
			t.Errorf("go.mod changed.\nbefore:\n%s\nafter:\n%s", before, after)
		}

		if errOut.Len() != 0 {
			t.Errorf("expected silence on a consistent module, got: %s", errOut.String())
		}
	})

	t.Run("absent require line is announced and added", func(t *testing.T) {
		root := newConsumerModule(t, "")

		var errOut bytes.Buffer

		runner.ExportEnsureTargDependency(runner.TargDependency{ModulePath: standInModule}, root, &errOut)

		if !strings.Contains(errOut.String(), "is not required by") {
			t.Errorf("expected an announcement on stderr, got: %q", errOut.String())
		}

		after, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			t.Fatalf("reading go.mod: %v", err)
		}

		if !strings.Contains(string(after), standInModule) {
			t.Errorf("expected %s to be added to go.mod, got:\n%s", standInModule, after)
		}
	})
}

// unexported constants.
const (
	// standInModule is a throwaway module path served only by the local file:// proxy.
	// Using a stand-in rather than github.com/toejough/targ keeps the test hermetic and
	// independent of what targ has published.
	standInModule = "example.com/standin"
)

// makeWritableOnCleanup registers a cleanup that makes every path under dir
// writable. The go module cache is written with read-only directories, so
// t.TempDir's own RemoveAll fails on it and reports a test error. Cleanups run
// LIFO, so registering this immediately after t.TempDir puts it ahead of the
// removal.
func makeWritableOnCleanup(t *testing.T, dir string) {
	t.Helper()

	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // best-effort cleanup
			}

			_ = os.Chmod(path, 0o700)

			return nil
		})
	})
}

// newConsumerModule creates a fresh module rooted at a new temp dir. When pin is
// non-empty the module requires standInModule at that version; when empty it
// requires nothing. The module always imports standInModule, so the require is a
// direct one and `go get` has no reason to annotate it `// indirect`. Returns the
// module root.
//
// A fresh directory per call is required: targ's binary cache is keyed on
// sha256(importRoot), so a reused path can silently skip the code under test.
func newConsumerModule(t *testing.T, pin string) string {
	t.Helper()

	root := t.TempDir()

	goMod := "module example.com/consumer\n\ngo 1.25.5\n"
	if pin != "" {
		goMod += "\nrequire " + standInModule + " " + pin + "\n"
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatalf("writing consumer go.mod: %v", err)
	}

	consumer := "package consumer\n\nimport _ \"" + standInModule + "\"\n"
	if err := os.WriteFile(filepath.Join(root, "consumer.go"), []byte(consumer), 0o600); err != nil {
		t.Fatalf("writing consumer.go: %v", err)
	}

	if pin != "" {
		// Populate go.sum so the fixture is consistent before the call under test.
		cmd := exec.Command("go", "mod", "download", standInModule)
		cmd.Dir = root

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("seeding go.sum: %v\n%s", err, out)
		}
	}

	return root
}

// newStandInProxy builds a file:// module proxy serving standInModule at v1.0.0 and
// v1.1.0, and redirects every module-resolution env var at it. The redirection is
// process-global, so callers need no handle -- creating the proxy IS the setup.
//
// The fixture pins v1.0.0 while the proxy also serves v1.1.0: an unversioned
// `go get` resolves to @upgrade and visibly moves the pin, which is what makes this
// a real regression test rather than a tautology.
//
// It uses t.Setenv, which forbids t.Parallel -- correct, because the Go module env is
// process-global. Integration tests here are serial by construction.
func newStandInProxy(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	makeWritableOnCleanup(t, dir)

	modDir := filepath.Join(dir, "proxy", standInModule, "@v")

	if err := os.MkdirAll(modDir, 0o750); err != nil {
		t.Fatalf("creating proxy dir: %v", err)
	}

	for _, v := range []string{"v1.0.0", "v1.1.0"} {
		writeProxyVersion(t, modDir, v)
	}

	if err := os.WriteFile(filepath.Join(modDir, "list"), []byte("v1.0.0\nv1.1.0\n"), 0o600); err != nil {
		t.Fatalf("writing proxy list: %v", err)
	}

	// GOPRIVATE/GONOPROXY are cleared, not set to standInModule: either one would
	// route the module around GOPROXY to a direct VCS fetch, i.e. the network.
	t.Setenv("GOPROXY", "file://"+filepath.Join(dir, "proxy"))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOPRIVATE", "")
	t.Setenv("GONOPROXY", "")
	t.Setenv("GONOSUMDB", "")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOMODCACHE", filepath.Join(dir, "modcache"))
	t.Setenv("GOPATH", filepath.Join(dir, "gopath"))
}

// writeModuleZip writes a minimal module zip at path. Entries must be prefixed
// "<module>@<version>/" or the go toolchain rejects the archive.
func writeModuleZip(t *testing.T, path, version, goMod string) {
	t.Helper()

	f, err := os.Create(path) //nolint:gosec // test fixture path from t.TempDir
	if err != nil {
		t.Fatalf("creating module zip: %v", err)
	}

	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	prefix := standInModule + "@" + version + "/"

	files := map[string]string{
		prefix + "go.mod":     goMod,
		prefix + "standin.go": "package standin\n",
	}

	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("creating zip entry %s: %v", name, err)
		}

		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("writing zip entry %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("closing module zip: %v", err)
	}
}

// writeProxyVersion writes the .info, .mod and .zip a file:// proxy needs for one
// version of standInModule.
func writeProxyVersion(t *testing.T, modDir, version string) {
	t.Helper()

	info := `{"Version":"` + version + `","Time":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(modDir, version+".info"), []byte(info), 0o600); err != nil {
		t.Fatalf("writing %s.info: %v", version, err)
	}

	mod := "module " + standInModule + "\n\ngo 1.25.5\n"
	if err := os.WriteFile(filepath.Join(modDir, version+".mod"), []byte(mod), 0o600); err != nil {
		t.Fatalf("writing %s.mod: %v", version, err)
	}

	writeModuleZip(t, filepath.Join(modDir, version+".zip"), version, mod)
}
