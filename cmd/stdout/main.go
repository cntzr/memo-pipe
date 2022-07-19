package main

import (
	"fmt"
	"os"
	"pipeline"
)

func main() {
	param := "short"
	if len(os.Args) > 1 {
		param = os.Args[1]
	}
	p := pipeline.Stdout(os.Stdin, param)
	p.Output = os.Stdout
	p.Stdout()
	if p.Error.Err != nil {
		fmt.Fprintln(os.Stderr, p.Error.Err.Error())
		os.Exit(1)
	}
}
