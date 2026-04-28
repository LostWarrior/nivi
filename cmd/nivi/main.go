package main

import (
	"os"

	"github.com/LostWarrior/nivi/internal/cli"
	niviruntime "github.com/LostWarrior/nivi/internal/runtime"
)

func main() {
	streams := niviruntime.IO{
		In:        os.Stdin,
		Out:       os.Stdout,
		Err:       os.Stderr,
		StdinTTY:  niviruntime.IsTTY(os.Stdin),
		StdoutTTY: niviruntime.IsTTY(os.Stdout),
	}

	os.Exit(cli.Run(os.Args[1:], streams))
}
