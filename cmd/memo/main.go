package main

import (
	"fmt"
	"os"
	"pipeline"
)

func main() {
	p := pipeline.ToJSON(os.Stdin)
	p.Output = os.Stdout
	p.Stdout()
	if p.Error.Err != nil {
		fmt.Fprintln(os.Stderr, p.Error.Err.Error())
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✓ converted")
}
