package main

import "fmt"

type Build struct {
	Target string `targ:"flag"`
}

func (b *Build) Run() {
	fmt.Printf("Building target: %s\n", b.Target)
}

type Clean struct{}

func (c *Clean) Run() {
	fmt.Println("Cleaning...")
}

type Deploy struct {
	Staging *StagingCmd `targ:"subcommand"`
	Prod    *ProdCmd    `targ:"subcommand"`
}

type ProdCmd struct{}

func (p *ProdCmd) Run() { fmt.Println("Deploying to Prod") }

type StagingCmd struct{}

func (s *StagingCmd) Run() { fmt.Println("Deploying to Staging") }
