package main

import (
	"github.com/toejough/targ"
)

func init() {
	targ.Register(
		targ.Targ(build).Name("build"),
		targ.Targ(clean).Name("clean"),
		targ.NewGroup("deploy",
			targ.Targ(deployStaging).Name("staging"),
			targ.Targ(deployProd).Name("prod"),
		),
	)
}

func main() {
	targ.ExecuteRegistered()
}
