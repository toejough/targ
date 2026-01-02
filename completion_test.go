package commander

import "testing"

func TestTokenizeCommandLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		parts    []string
		isNewArg bool
	}{
		{
			name:     "simple split",
			input:    "cmd build -t tag",
			parts:    []string{"cmd", "build", "-t", "tag"},
			isNewArg: false,
		},
		{
			name:     "trailing space",
			input:    "cmd build ",
			parts:    []string{"cmd", "build"},
			isNewArg: true,
		},
		{
			name:     "double quoted",
			input:    "cmd \"two words\"",
			parts:    []string{"cmd", "two words"},
			isNewArg: false,
		},
		{
			name:     "single quoted",
			input:    "cmd 'two words'",
			parts:    []string{"cmd", "two words"},
			isNewArg: false,
		},
		{
			name:     "escaped space",
			input:    "cmd two\\ words",
			parts:    []string{"cmd", "two words"},
			isNewArg: false,
		},
		{
			name:     "unfinished quote",
			input:    "cmd \"unterminated",
			parts:    []string{"cmd", "unterminated"},
			isNewArg: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parts, isNewArg := tokenizeCommandLine(test.input)
			if len(parts) != len(test.parts) {
				t.Fatalf("expected %d parts, got %d", len(test.parts), len(parts))
			}
			for i := range parts {
				if parts[i] != test.parts[i] {
					t.Fatalf("expected part %d to be %q, got %q", i, test.parts[i], parts[i])
				}
			}
			if isNewArg != test.isNewArg {
				t.Fatalf("expected isNewArg=%v, got %v", test.isNewArg, isNewArg)
			}
		})
	}
}
