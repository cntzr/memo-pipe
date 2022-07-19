package main

import (
	"os"
	"pipeline"
)

func main() {
	pipeline.Keep(os.Stdin)
}
