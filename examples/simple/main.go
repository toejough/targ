package main

import (
	"fmt"
	"github.com/toejough/targ"
)

type Greet struct {
	Name string `targ:"required,desc=Name of the person to greet"`
	Age  int    `targ:"flag,name=age,desc=Age of the person"`
}

// Greet the user.
// This command prints a greeting message.
func (g *Greet) Run() {
	fmt.Printf("Hello %s, you are %d years old!\n", g.Name, g.Age)
}

type Math struct {
	Add    *AddCmd `targ:"subcommand"`
	RunCmd *RunCmd `targ:"subcommand=run"`
}

// Math operations.
// Provides basic math commands.
func (m *Math) Run() {
	fmt.Printf("example with just calling `math`!\n")
}

type AddCmd struct {
	A int `targ:"positional"`
	B int `targ:"positional"`
}

// Add two numbers.
func (a *AddCmd) Run() {
	fmt.Printf("%d + %d = %d\n", a.A, a.B, a.A+a.B)
}

// Example of a subcommand named "run"
// Usage: math run
type RunCmd struct{}

// Execute the run command.
func (r *RunCmd) Run() {
	fmt.Println("Math run command executed")
}

func main() {
	targ.Run(Greet{}, Math{})
}
