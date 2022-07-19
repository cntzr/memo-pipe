package main

import (
	"fmt"
	"os"
	"pipeline"
)

func main() {
	param := "help"
	if len(os.Args) > 1 {
		param = os.Args[1]
	}
	p := pipeline.From(pipeline.Period(param), "")
	p.Output = os.Stdout
	p.Stdout()
	if p.Error.Err != nil {
		fmt.Fprintln(os.Stderr, p.Error.Err.Error())
		os.Exit(1)
	}
}
