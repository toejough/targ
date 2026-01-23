// Package main demonstrates manifest-based targ usage with the Target/Group model.
package main

import (
	"fmt"

	"github.com/toejough/targ"
)

func main() {
	targ.ExecuteRegistered()
}

func init() {
	targ.Register(
		targ.NewGroup("build",
			targ.Targ(buildDocker).Name("docker").Description("Build Docker image"),
			targ.Targ(buildLinux).Name("linux").Description("Build Linux binary"),
		),
		targ.Targ(deploy).Name("deploy").Description("Deployment commands"),
	)
}

type BuildDockerArgs struct {
	Tag string `targ:"flag,desc=Docker image tag"`
}

type DeployArgs struct {
	Env string `targ:"flag,desc=Deployment environment"`
}

func buildDocker(args BuildDockerArgs) {
	fmt.Printf("Building Docker image with tag: %s\n", args.Tag)
}

func buildLinux() {
	fmt.Println("Building Linux binary")
}

func deploy(args DeployArgs) {
	fmt.Printf("Deploying to %s...\n", args.Env)
}
