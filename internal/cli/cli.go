// Package cli holds the commands of the avero binary, S14. The command in
// cmd/avero is a thin wrapper, so a test drives the commands with no process.
package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/alternayte/avero/assets"
)

// Streams holds the input and the output of one run.
type Streams struct {
	// Out receives the result of a command.
	Out io.Writer
	// Err receives a fault.
	Err io.Writer
	// In carries the requests of `avero mcp`. A nil value reads the standard
	// input.
	In io.Reader
	// Dir is the working directory. An empty value is the process
	// directory.
	Dir string
	// Fetch reads the package of a library. A nil value reads over HTTP. A
	// test sets it, so the gate needs no network.
	Fetch assets.Fetcher
}

// command is one command of the binary.
type command struct {
	// Name is the word that the person types.
	Name string
	// Usage is the one line form of the command.
	Usage string
	// Summary states what the command does, in one line.
	Summary string
	// Run performs the command and returns the exit code.
	Run func(ctx context.Context, s Streams, args []string) int
}

// commands holds every command that the binary carries. The list is built in
// an initialiser, because the help command reads the list itself.
var commands []command

// init fills the command list.
func init() {
	commands = []command{
		{Name: "new", Usage: "avero new <name> [--shape ssr|spa|api] [--hypermedia datastar|htmx] [--module <path>] [--replace <dir>]",
			Summary: "write a new application", Run: runNew},
		{Name: "generate", Usage: "avero generate [--check]",
			Summary: "write the generated files of this application", Run: runGenerate},
		{Name: "slice", Usage: "avero slice <name>",
			Summary: "write a new feature slice", Run: runSlice},
		{Name: "dev", Usage: "avero dev [--port 8080]",
			Summary: "run the application and rebuild it on each change", Run: runDev},
		{Name: "build", Usage: "avero build [--minify]",
			Summary: "generate, build the assets and compile the binary", Run: runBuild},
		{Name: "migrate", Usage: "avero migrate new <name>|up|down|status",
			Summary: "write and apply the migrations", Run: runMigrate},
		{Name: "routes", Usage: "avero routes [--json] [--openapi [--out openapi.json] [--server https://api.example.com]]",
			Summary: "print the routes of this application, or its OpenAPI description", Run: runRoutes},
		{Name: "modules", Usage: "avero modules [--json]",
			Summary: "print the contribution of each module", Run: runModules},
		{Name: "schema", Usage: "avero schema [--json]",
			Summary: "print the models that the modules describe", Run: runSchema},
		{Name: "doctor", Usage: "avero doctor [--json]",
			Summary: "prove the configuration, the database and the broker", Run: runDoctor},
		{Name: "verify", Usage: "avero verify [--json]",
			Summary: "run the gate of this application", Run: runVerify},
		{Name: "js", Usage: "avero js pin <package> [url]",
			Summary: "fetch a bundled module into the vendor directory", Run: runJS},
		{Name: "ui", Usage: "avero ui add <library> [--style <name>]",
			Summary: "add a component library to this application", Run: runUI},
		{Name: "assets", Usage: "avero assets init",
			Summary: "write the package.json of tier 1", Run: runAssets},
		{Name: "mcp", Usage: "avero mcp",
			Summary: "serve the agent surface over stdio", Run: runMCP},
		{Name: "version", Usage: "avero version", Summary: "print the version", Run: runVersion},
		{Name: "help", Usage: "avero help [command]", Summary: "print this text", Run: runHelp},
	}
}

// later names the commands that the SDD states and a later subsystem carries.
// The binary does not hold them yet, and help says which subsystem adds each
// one. A command that does nothing would be worse than an absent one.
var later = []struct{ Name, Subsystem string }{
	{"avero outbox dead --list|--replay", "S7, the outbox relay"},
	{"avero inbox dead --list|--replay", "S8, the inbox consumer"},
	{"avero es replay|streams|checkpoints", "S9, the event sourcing"},
}

// Run performs one command and returns the exit code of the process.
func Run(ctx context.Context, s Streams, args []string) int {
	if len(args) == 0 {
		runHelp(ctx, s, nil)
		return 1
	}
	name := args[0]
	for _, c := range commands {
		if c.Name == name {
			return c.Run(ctx, s, args[1:])
		}
	}
	for _, l := range later {
		if strings.HasPrefix(l.Name, "avero "+name+" ") {
			return failf(s, "the command %q needs a subsystem that does not exist yet: %s",
				name, l.Subsystem)
		}
	}
	return failf(s, "the command %q is not known\n  → Run `avero help` to see every command", name)
}

// runHelp prints the commands.
func runHelp(_ context.Context, s Streams, args []string) int {
	if len(args) > 0 {
		for _, c := range commands {
			if c.Name == args[0] {
				_, _ = fmt.Fprintf(s.Out, "%s\n\n%s\n", c.Usage, c.Summary)
				return 0
			}
		}
		return failf(s, "the command %q is not known", args[0])
	}
	var b strings.Builder
	b.WriteString("avero is the application host for Go services.\n\nUsage:\n\n\tavero <command> [arguments]\n\nThe commands are:\n\n")
	rows := make([]command, len(commands))
	copy(rows, commands)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	for _, c := range rows {
		fmt.Fprintf(&b, "\t%-10s %s\n", c.Name, c.Summary)
	}
	b.WriteString("\nThese commands arrive with a later subsystem:\n\n")
	for _, l := range later {
		fmt.Fprintf(&b, "\t%-40s %s\n", l.Name, l.Subsystem)
	}
	b.WriteString("\nRun `avero help <command>` for the form of one command.\n")
	_, _ = io.WriteString(s.Out, b.String())
	return 0
}

// Version is the version of the binary. The release build sets it.
var Version = "dev"

// runVersion prints the version.
func runVersion(_ context.Context, s Streams, _ []string) int {
	_, _ = fmt.Fprintln(s.Out, "avero "+Version)
	return 0
}

// hint returns the repair line of a fault.
//
// An error carries one sentence that states the repair. That sentence ends
// with a full stop. The linter refuses an error literal that ends with
// punctuation, so the sentence arrives as a value. See DX-7.
func hint(sentence string) string { return "\n  → " + sentence }

// failf writes a fault and returns the exit code.
func failf(s Streams, format string, args ...any) int {
	_, _ = fmt.Fprintf(s.Err, format+"\n", args...)
	return 1
}

// fail writes an error and returns the exit code. It returns 0 for a nil
// error, so a command reads as one line.
func fail(s Streams, err error) int {
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintln(s.Err, err)
	return 1
}
