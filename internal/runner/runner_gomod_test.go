package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/toejough/targ/internal/runner"
)

func TestPinnedModuleVersion(t *testing.T) {
	t.Parallel()

	const target = "github.com/toejough/targ"

	cases := []struct {
		name  string
		goMod string // "" means: do not create a go.mod at all
		want  string
	}{
		{
			name: "require inside a block",
			goMod: `module example.com/consumer

go 1.25.5

require (
	github.com/toejough/targ v0.0.0-20260724192707-d0eae3bdf0de
	golang.org/x/mod v0.32.0
)
`,
			want: "v0.0.0-20260724192707-d0eae3bdf0de",
		},
		{
			name: "require on its own line",
			goMod: `module example.com/consumer

go 1.25.5

require github.com/toejough/targ v1.2.3
`,
			want: "v1.2.3",
		},
		{
			name: "module absent from go.mod",
			goMod: `module example.com/consumer

go 1.25.5

require golang.org/x/mod v0.32.0
`,
			want: "",
		},
		{
			name:  "go.mod absent",
			goMod: "",
			want:  "",
		},
		{
			name: "go.mod unparseable",
			goMod: `module example.com/consumer
require ( github.com/toejough/targ
`,
			want: "",
		},
		{
			name: "module present only as a replace target",
			goMod: `module example.com/consumer

go 1.25.5

require golang.org/x/mod v0.32.0

replace github.com/toejough/targ => ../targ
`,
			want: "",
		},
		{
			name: "require carries an indirect comment",
			goMod: `module example.com/consumer

go 1.25.5

require github.com/toejough/targ v0.9.0 // indirect
`,
			want: "v0.9.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Each subtest gets its own directory. Never share a fixture dir.
			dir := t.TempDir()
			if tc.goMod != "" {
				path := filepath.Join(dir, "go.mod")
				if err := os.WriteFile(path, []byte(tc.goMod), 0o600); err != nil {
					t.Fatalf("writing fixture go.mod: %v", err)
				}
			}

			got := runner.ExportPinnedModuleVersion(dir, target)
			if got != tc.want {
				t.Errorf("ExportPinnedModuleVersion(%q, %q) = %q, want %q", dir, target, got, tc.want)
			}
		})
	}
}
