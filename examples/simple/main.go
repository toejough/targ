// Package main demonstrates simple targ usage with the Target/Group model.
package main

import (
	"fmt"

	"github.com/toejough/targ"
)

func main() {
	targ.ExecuteRegistered()
}

//nolint:gochecknoinits // init required for targ.Register pattern - targets must register before main runs
func init() {
	targ.Register(
		targ.Targ(greet).Name("greet"),
		targ.Group("math",
			targ.Targ(add).Name("add"),
			targ.Targ(mathRun).Name("run"),
		),
	)
}

type AddArgs struct {
	A int `targ:"positional"`
	B int `targ:"positional"`
}

type GreetArgs struct {
	Name string `targ:"required,desc=Name of the person to greet"`
	Age  int    `targ:"flag,name=age,desc=Age of the person"`
}

func add(args AddArgs) {
	fmt.Printf("%d + %d = %d\n", args.A, args.B, args.A+args.B)
}

func greet(args GreetArgs) {
	fmt.Printf("Hello %s, you are %d years old!\n", args.Name, args.Age)
}

func mathRun() {
	fmt.Println("Math run command executed")
}
