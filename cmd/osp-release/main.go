package main

import (
	"context"
	"os"

	"github.com/openshift-pipelines/osp-release/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Stdout, os.Stderr, cli.IsTerminal(os.Stdout)))
}
