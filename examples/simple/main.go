package main

import (
	"commander"
	"fmt"
)

type Greet struct {
	Name string `commander:"required,desc=Name of the person to greet"`
	Age  int    `commander:"flag,name=age,desc=Age of the person"`
}

func (g *Greet) Run() {
	fmt.Printf("Hello %s, you are %d years old!\n", g.Name, g.Age)
}

type Math struct {
	Add    *AddCmd `commander:"subcommand,desc=Add two numbers"`
	RunCmd *RunCmd `commander:"subcommand=run,desc=Run command"`
}

func (m *Math) Run() {
	fmt.Printf("example with just calling `math`!\n")
}

type AddCmd struct {
	A int `commander:"positional"`
	B int `commander:"positional"`
}

func (a *AddCmd) Run() {
	fmt.Printf("%d + %d = %d\n", a.A, a.B, a.A+a.B)
}

// Example of a subcommand named "run"
// Usage: math run
type RunCmd struct{}

func (r *RunCmd) Run() {
	fmt.Println("Math run command executed")
}

func main() {
	commander.Run(Greet{}, Math{})
}
