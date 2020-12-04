package main

import (
	"os"

	"github.com/TierMobility/boring-registry/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
