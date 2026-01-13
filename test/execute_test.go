package targ_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/toejough/targ"
)

type ExecuteDefaultCmd struct {
	Called bool
}

func (c *ExecuteDefaultCmd) Run() {
	c.Called = true
}

type ExecuteErrorCmd struct{}

func (c *ExecuteErrorCmd) Run() error {
	return fmt.Errorf("command failed")
}

// --- Basic Execute Tests ---

type ExecuteTestCmd struct {
	Name   string `targ:"flag"`
	Called bool
}

func (c *ExecuteTestCmd) Run() {
	c.Called = true
}

type FastCmd struct {
	Called bool
}

func (c *FastCmd) Run(ctx context.Context) error {
	c.Called = true
	return nil
}

// --- Interleaved Flags Tests ---

type InterleavedFlagsCmd struct {
	Include []targ.Interleaved[string] `targ:"flag"`
	Exclude []targ.Interleaved[string] `targ:"flag"`
}

func (c *InterleavedFlagsCmd) Run() {}

type InterleavedIntCmd struct {
	Values []targ.Interleaved[int] `targ:"flag"`
}

func (c *InterleavedIntCmd) Run() {}

type MapStringIntCmd struct {
	Ports map[string]int `targ:"flag"`
}

func (c *MapStringIntCmd) Run() {}

// --- Map Flags Tests ---

type MapStringStringCmd struct {
	Labels map[string]string `targ:"flag"`
}

func (c *MapStringStringCmd) Run() {}

// --- Repeated Flags Tests ---

type RepeatedFlagsCmd struct {
	Tags []string `targ:"flag"`
}

func (c *RepeatedFlagsCmd) Run() {}

type RepeatedIntFlagsCmd struct {
	Counts []int `targ:"flag"`
}

func (c *RepeatedIntFlagsCmd) Run() {}

type SlowCmd struct {
	Called bool
}

func (c *SlowCmd) Run(ctx context.Context) error {
	c.Called = true
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Second):
		return nil
	}
}

// --- Timeout Tests ---

type TimeoutCmd struct {
	Called bool
}

func (c *TimeoutCmd) Run(ctx context.Context) error {
	c.Called = true
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

func TestExecuteWithOptions_AllowDefault(t *testing.T) {
	cmd := &ExecuteDefaultCmd{}
	_, err := targ.ExecuteWithOptions([]string{"app"}, targ.RunOptions{AllowDefault: true}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd.Called {
		t.Fatal("expected default command to be called")
	}
}

func TestExecuteWithOptions_NoDefaultShowsUsage(t *testing.T) {
	cmd := &ExecuteDefaultCmd{}
	_, err := targ.ExecuteWithOptions([]string{"app"}, targ.RunOptions{AllowDefault: false}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Called {
		t.Fatal("expected command NOT to be called without AllowDefault")
	}
}

func TestExecute_CommandError(t *testing.T) {
	cmd := &ExecuteErrorCmd{}
	result, err := targ.Execute([]string{"app"}, cmd)
	if err == nil {
		t.Fatal("expected error from command")
	}
	exitErr, ok := err.(targ.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
	if !strings.Contains(result.Output, "command failed") {
		t.Fatalf("expected error message in output, got: %q", result.Output)
	}
}

func TestExecute_Success(t *testing.T) {
	cmd := &ExecuteTestCmd{}
	_, err := targ.Execute([]string{"app", "--name", "Alice"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
	if cmd.Name != "Alice" {
		t.Fatalf("expected name=Alice, got %q", cmd.Name)
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	cmd := &ExecuteTestCmd{}
	result, err := targ.ExecuteWithOptions([]string{"app", "unknown"}, targ.RunOptions{AllowDefault: false}, cmd)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	exitErr, ok := err.(targ.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
	if !strings.Contains(result.Output, "Unknown command") {
		t.Fatalf("expected unknown command message, got: %q", result.Output)
	}
}

func TestInterleavedFlags_IntType(t *testing.T) {
	cmd := &InterleavedIntCmd{}
	_, err := targ.Execute([]string{"app", "--values", "10", "--values", "20"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(cmd.Values))
	}
	if cmd.Values[0].Value != 10 || cmd.Values[0].Position != 0 {
		t.Fatalf("expected values[0]={10,0}, got %+v", cmd.Values[0])
	}
	if cmd.Values[1].Value != 20 || cmd.Values[1].Position != 1 {
		t.Fatalf("expected values[1]={20,1}, got %+v", cmd.Values[1])
	}
}

func TestInterleavedFlags_ReconstructOrder(t *testing.T) {
	cmd := &InterleavedFlagsCmd{}
	_, err := targ.Execute([]string{"app", "--exclude", "x", "--include", "a", "--include", "b", "--exclude", "y"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type item struct {
		isInclude bool
		value     string
		position  int
	}
	var all []item
	for _, inc := range cmd.Include {
		all = append(all, item{true, inc.Value, inc.Position})
	}
	for _, exc := range cmd.Exclude {
		all = append(all, item{false, exc.Value, exc.Position})
	}

	// Sort by position
	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].position < all[i].position {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	expected := []item{
		{false, "x", 0},
		{true, "a", 1},
		{true, "b", 2},
		{false, "y", 3},
	}
	if len(all) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(all))
	}
	for i, exp := range expected {
		if all[i] != exp {
			t.Fatalf("item[%d]: expected %+v, got %+v", i, exp, all[i])
		}
	}
}

func TestInterleavedFlags_TracksPosition(t *testing.T) {
	cmd := &InterleavedFlagsCmd{}
	_, err := targ.Execute([]string{"app", "--include", "a", "--exclude", "b", "--include", "c"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Include) != 2 {
		t.Fatalf("expected 2 includes, got %d: %v", len(cmd.Include), cmd.Include)
	}
	if cmd.Include[0].Value != "a" || cmd.Include[0].Position != 0 {
		t.Fatalf("expected include[0]={a,0}, got %+v", cmd.Include[0])
	}
	if cmd.Include[1].Value != "c" || cmd.Include[1].Position != 2 {
		t.Fatalf("expected include[1]={c,2}, got %+v", cmd.Include[1])
	}

	if len(cmd.Exclude) != 1 {
		t.Fatalf("expected 1 exclude, got %d: %v", len(cmd.Exclude), cmd.Exclude)
	}
	if cmd.Exclude[0].Value != "b" || cmd.Exclude[0].Position != 1 {
		t.Fatalf("expected exclude[0]={b,1}, got %+v", cmd.Exclude[0])
	}
}

func TestMapFlags_InvalidFormat(t *testing.T) {
	cmd := &MapStringStringCmd{}
	_, err := targ.Execute([]string{"app", "--labels", "invalid"}, cmd)
	if err == nil {
		t.Fatal("expected error for invalid map format")
	}
}

func TestMapFlags_OverwriteKey(t *testing.T) {
	cmd := &MapStringStringCmd{}
	_, err := targ.Execute([]string{"app", "--labels", "env=dev", "--labels", "env=prod"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Labels["env"] != "prod" {
		t.Fatalf("expected env=prod (overwritten), got %q", cmd.Labels["env"])
	}
}

func TestMapFlags_StringInt(t *testing.T) {
	cmd := &MapStringIntCmd{}
	_, err := targ.Execute([]string{"app", "--ports", "http=80", "--ports", "https=443"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d: %v", len(cmd.Ports), cmd.Ports)
	}
	if cmd.Ports["http"] != 80 {
		t.Fatalf("expected http=80, got %d", cmd.Ports["http"])
	}
	if cmd.Ports["https"] != 443 {
		t.Fatalf("expected https=443, got %d", cmd.Ports["https"])
	}
}

func TestMapFlags_StringString(t *testing.T) {
	cmd := &MapStringStringCmd{}
	_, err := targ.Execute([]string{"app", "--labels", "env=prod", "--labels", "app=web"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(cmd.Labels), cmd.Labels)
	}
	if cmd.Labels["env"] != "prod" {
		t.Fatalf("expected env=prod, got %q", cmd.Labels["env"])
	}
	if cmd.Labels["app"] != "web" {
		t.Fatalf("expected app=web, got %q", cmd.Labels["app"])
	}
}

func TestMapFlags_ValueWithEquals(t *testing.T) {
	cmd := &MapStringStringCmd{}
	_, err := targ.Execute([]string{"app", "--labels", "equation=a=b"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Labels["equation"] != "a=b" {
		t.Fatalf("expected equation=a=b, got %q", cmd.Labels["equation"])
	}
}

func TestRepeatedFlags_Accumulates(t *testing.T) {
	cmd := &RepeatedFlagsCmd{}
	_, err := targ.Execute([]string{"app", "--tags", "a", "--tags", "b", "--tags", "c"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(cmd.Tags), cmd.Tags)
	}
	if cmd.Tags[0] != "a" || cmd.Tags[1] != "b" || cmd.Tags[2] != "c" {
		t.Fatalf("unexpected tags order: %v", cmd.Tags)
	}
}

func TestRepeatedFlags_IntSlice(t *testing.T) {
	cmd := &RepeatedIntFlagsCmd{}
	_, err := targ.Execute([]string{"app", "--counts", "1", "--counts", "2", "--counts", "3"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Counts) != 3 {
		t.Fatalf("expected 3 counts, got %d: %v", len(cmd.Counts), cmd.Counts)
	}
	if cmd.Counts[0] != 1 || cmd.Counts[1] != 2 || cmd.Counts[2] != 3 {
		t.Fatalf("unexpected counts: %v", cmd.Counts)
	}
}

func TestTimeout_EqualsStyle(t *testing.T) {
	cmd := &TimeoutCmd{}
	_, err := targ.Execute([]string{"app", "--timeout=1s"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_Exceeded(t *testing.T) {
	cmd := &SlowCmd{}
	_, err := targ.Execute([]string{"app", "--timeout", "10ms"}, cmd)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestTimeout_GlobalAndPerCommand(t *testing.T) {
	cmd := &TimeoutCmd{}
	_, err := targ.ExecuteWithOptions(
		[]string{"app", "--timeout", "5s", "timeout-cmd", "--timeout", "1s"},
		targ.RunOptions{AllowDefault: false},
		cmd,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_InvalidDuration(t *testing.T) {
	cmd := &TimeoutCmd{}
	_, err := targ.Execute([]string{"app", "--timeout", "invalid"}, cmd)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestTimeout_MultiCommandDifferentTimeouts(t *testing.T) {
	fast := &FastCmd{}
	slow := &SlowCmd{}
	_, err := targ.ExecuteWithOptions(
		[]string{"app", "fast-cmd", "--timeout", "10ms", "slow-cmd", "--timeout", "2s"},
		targ.RunOptions{AllowDefault: false},
		fast, slow,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fast.Called {
		t.Fatal("expected fast command to be called")
	}
	if !slow.Called {
		t.Fatal("expected slow command to be called")
	}
}

func TestTimeout_NotExceeded(t *testing.T) {
	cmd := &TimeoutCmd{}
	_, err := targ.Execute([]string{"app", "--timeout", "1s"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_PerCommand(t *testing.T) {
	cmd := &TimeoutCmd{}
	_, err := targ.ExecuteWithOptions(
		[]string{"app", "timeout-cmd", "--timeout", "1s"},
		targ.RunOptions{AllowDefault: false},
		cmd,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd.Called {
		t.Fatal("expected command to be called")
	}
}

func TestTimeout_PerCommandExceeded(t *testing.T) {
	cmd := &SlowCmd{}
	_, err := targ.ExecuteWithOptions(
		[]string{"app", "slow-cmd", "--timeout", "10ms"},
		targ.RunOptions{AllowDefault: false},
		cmd,
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
