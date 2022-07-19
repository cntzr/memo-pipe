package main

import (
	"fmt"
	"os"
	"pipeline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Sorry ... maybe you forgot any tag?")
		os.Exit(1)
	}
	p := pipeline.Tagged(os.Stdin, os.Args[1:]...)
	p.Output = os.Stdout
	p.Stdout()
	if p.Error.Err != nil {
		fmt.Fprintln(os.Stderr, p.Error.Err.Error())
		os.Exit(1)
	}

}
