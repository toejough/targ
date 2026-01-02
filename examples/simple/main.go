package main

import (
	"fmt"
	"commander"
)

type Greet struct {
	Name string `commander:"required,desc=Name of the person to greet"`
	Age  int    `commander:"flag,name=age,desc=Age of the person"`
}

func (g *Greet) Run() {
	fmt.Printf("Hello %s, you are %d years old!\n", g.Name, g.Age)
}

type Math struct {}

type AddArgs struct {
	A int `commander:"positional"`
	B int `commander:"positional"`
}

func (m Math) Add(args AddArgs) {
	fmt.Printf("%d + %d = %d\n", args.A, args.B, args.A+args.B)
}

// Example of a subcommand named "run"
// Usage: math run
type RunArgs struct {}
func (m Math) Run(args RunArgs) {
	fmt.Println("Math run command executed")
}

func main() {
	commander.Run(&Greet{}, Math{})
}
