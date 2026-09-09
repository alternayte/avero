package cli

import (
	"context"
	"io"
	"os"

	"github.com/alternayte/avero/mcp"
)

// runMCP serves the agent surface over stdio.
func runMCP(ctx context.Context, s Streams, args []string) int {
	if len(args) > 0 {
		return failf(s, "avero mcp: the flag %q is not known\n  → Run `avero mcp`, which serves the Model Context Protocol over stdio", args[0])
	}
	server := &mcp.Server{
		Dir:      dirOf(s),
		Slice:    Slice,
		Generate: GenerateCheck,
	}
	// The protocol owns the output of the command, so a log line would break
	// the transport. The server writes nothing else.
	var in io.Reader = os.Stdin
	if s.In != nil {
		in = s.In
	}
	if err := server.Serve(ctx, in, s.Out); err != nil {
		return fail(s, err)
	}
	return 0
}
