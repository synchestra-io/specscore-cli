package main

import (
	"os"

	"github.com/specscore/specscore-cli/internal/cli"
)

func main() {
	if err := cli.Run(os.Args); err != nil {
		cli.Fatal(err)
	}
}
