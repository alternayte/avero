// Command averogen writes the binding and the validation of each handler
// input, and the implementation of each client interface. S14 folds it into
// `avero generate`.
//
//	go run github.com/alternayte/avero/internal/cmd/averogen [-check] [dir]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
)

func main() {
	check := flag.Bool("check", false, "report a generated file that is not current and write nothing")
	flag.Parse()

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	if *check {
		for _, err := range []error{codegen.Check(dir), client.Check(dir)} {
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		return
	}

	handlers, err := codegen.Generate(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	clients, err := client.Generate(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, path := range append(handlers, clients...) {
		fmt.Println(path)
	}
}
