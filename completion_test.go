package commander

import "testing"

type EnumCmd struct {
	Mode string `commander:"flag,enum=dev|prod,short=m"`
	Kind string `commander:"flag,enum=fast|slow"`
}

func (c *EnumCmd) Run() {}

func TestEnumValuesForArg_LongFlag(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values, ok := enumValuesForArg(cmd, []string{"--mode"}, "", true)
	if !ok {
		t.Fatal("expected enum values for --mode")
	}
	if len(values) != 2 || values[0] != "dev" || values[1] != "prod" {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestEnumValuesForArg_ShortFlag(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	values, ok := enumValuesForArg(cmd, []string{"-m"}, "", true)
	if !ok {
		t.Fatal("expected enum values for -m")
	}
	if len(values) != 2 || values[0] != "dev" || values[1] != "prod" {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestEnumValuesForArg_NoMatch(t *testing.T) {
	cmd, err := parseCommand(&EnumCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if values, ok := enumValuesForArg(cmd, []string{"--unknown"}, "", true); ok {
		t.Fatalf("expected no enum values, got %v", values)
	}
}
