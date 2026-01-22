// Package main demonstrates CLI mode usage with the Target/Group model.
package main

import "fmt"

// --- Build ---

type BuildArgs struct {
	Target string `targ:"flag"`
}

func build(args BuildArgs) {
	fmt.Printf("Building target: %s\n", args.Target)
}

// --- Clean ---

func clean() {
	fmt.Println("Cleaning...")
}

func deployProd() {
	fmt.Println("Deploying to Prod")
}

// --- Deploy ---

func deployStaging() {
	fmt.Println("Deploying to Staging")
}
