package main

import (
	"github.com/toejough/targ"
)

func main() {
	targ.ExecuteRegistered()
}

//nolint:gochecknoinits // init required for targ.Register pattern - targets must register before main runs
func init() {
	targ.Register(
		targ.Targ(build).Name("build"),
		targ.Targ(clean).Name("clean"),
		targ.Group("deploy",
			targ.Targ(deployStaging).Name("staging"),
			targ.Targ(deployProd).Name("prod"),
		),
	)
}
