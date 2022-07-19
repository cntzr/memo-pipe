package main

import (
	"fmt"
	"os"
	"pipeline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Sorry ... maybe you forgot the tag?")
		os.Exit(1)
	}
	tag := os.Args[1]
	p := pipeline.TagIt(os.Stdin, tag)
	p.Output = os.Stdout
	p.Stdout()
	if p.Error.Err != nil {
		fmt.Fprintln(os.Stderr, p.Error.Err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ tagged ... %s\n", tag)
}
