package main

import (
	"os"

	"github.com/specscore/specscore-cli/internal/cli"
)

func main() {
	run(os.Args, cli.Run, cli.Fatal)
}

// run is the testable core of main. It calls runFn with args and, on error,
// passes the error to fatalFn.
func run(args []string, runFn func([]string) error, fatalFn func(error)) {
	if err := runFn(args); err != nil {
		fatalFn(err)
	}
}
