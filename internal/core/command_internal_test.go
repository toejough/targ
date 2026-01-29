package core

import "testing"

func TestCompletionExampleWithGetenv(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		shell  string
		expect string
	}{
		{name: "Zsh", shell: "/bin/zsh", expect: "source <(targ --completion)"},
		{name: "Fish", shell: "/usr/bin/fish", expect: "targ --completion | source"},
		{name: "BashDefault", shell: "", expect: "eval \"$(targ --completion)\""},
		{name: "OtherShellFallsBack", shell: "/bin/nu", expect: "eval \"$(targ --completion)\""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			getenv := func(key string) string {
				if key == "SHELL" {
					return tc.shell
				}

				return ""
			}

			example := completionExampleWithGetenv(getenv)
			if example.Code != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, example.Code)
			}
		})
	}
}
