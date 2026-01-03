package main

import "fmt"

type Build struct {
	Target string `targs:"flag"`
}

func (b *Build) Run() {
	fmt.Printf("Building target: %s\n", b.Target)
}

type Clean struct{}

func (c *Clean) Run() {
	fmt.Println("Cleaning...")
}

type Deploy struct {
	Staging *StagingCmd `targs:"subcommand"`
	Prod    *ProdCmd    `targs:"subcommand"`
}

type StagingCmd struct{}

func (s *StagingCmd) Run() { fmt.Println("Deploying to Staging") }

type ProdCmd struct{}

func (p *ProdCmd) Run() { fmt.Println("Deploying to Prod") }
