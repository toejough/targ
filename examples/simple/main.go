// Package main demonstrates simple targ usage.
package main

import (
	"fmt"

	"github.com/toejough/targ"
)

func main() {
	targ.Run(Greet{}, Math{})
}

type AddCmd struct {
	A int `targ:"positional"`
	B int `targ:"positional"`
}

// Run adds two numbers.
func (a *AddCmd) Run() {
	fmt.Printf("%d + %d = %d\n", a.A, a.B, a.A+a.B)
}

type Greet struct {
	Name string `targ:"required,desc=Name of the person to greet"`
	Age  int    `targ:"flag,name=age,desc=Age of the person"`
}

// Run greets the user.
func (g *Greet) Run() {
	fmt.Printf("Hello %s, you are %d years old!\n", g.Name, g.Age)
}

type Math struct {
	Add    *AddCmd `targ:"subcommand"`
	RunCmd *RunCmd `targ:"subcommand=run"`
}

// Run displays math options.
func (m *Math) Run() {
	fmt.Printf("example with just calling `math`!\n")
}

// RunCmd is a subcommand named "run" (usage: math run).
type RunCmd struct{}

// Run executes the run command.
func (r *RunCmd) Run() {
	fmt.Println("Math run command executed")
}
