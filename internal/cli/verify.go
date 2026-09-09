package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
	"github.com/alternayte/avero/verify"
)

// runVerify runs the gate of the application.
func runVerify(ctx context.Context, s Streams, args []string) int {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json", "-json":
			asJSON = true
		default:
			return failf(s, "avero verify: the flag %q is not known\n  → Run the command with --json or with no flag", a)
		}
	}
	rep := verify.Run(ctx, dirOf(s), verify.Steps(GenerateCheck))
	if asJSON {
		enc := json.NewEncoder(s.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return fail(s, err)
		}
	} else {
		_, _ = fmt.Fprint(s.Out, rep.String())
	}
	if !rep.OK {
		return 1
	}
	return 0
}

// GenerateCheck proves that every generated file of the application is
// current. It runs in this process, so the application needs no dependency on
// the avero binary. The MCP server calls the same function. See S16.
func GenerateCheck(dir string) error {
	if err := codegen.Check(dir); err != nil {
		return err
	}
	return client.Check(dir)
}
