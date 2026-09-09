// Command avero writes an application, generates its code, proves its
// configuration and builds it. See the SDD, S14.
//
//	avero new <name> [--shape ssr|spa|api] [--hypermedia datastar|htmx]
//	avero help
package main

import (
	"context"
	"os"

	"github.com/alternayte/avero/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), cli.Streams{Out: os.Stdout, Err: os.Stderr}, os.Args[1:]))
}
