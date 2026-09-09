package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/alternayte/avero/internal/inspect"
	"github.com/alternayte/avero/verify"
)

// Tool is one tool of the server.
type Tool struct {
	// Name is the name that a caller sends.
	Name string `json:"name"`
	// Description states what the tool returns, in one line.
	Description string `json:"description"`
	// InputSchema states the arguments of the tool.
	InputSchema map[string]any `json:"inputSchema"`
	// run performs the tool and returns the structured result.
	run func(ctx context.Context, s *Server, args json.RawMessage) (any, error)
}

// noArguments is the schema of a tool that takes none.
func noArguments() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
}

// oneString is the schema of a tool that takes one string.
func oneString(name, description string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			name: map[string]any{"type": "string", "description": description},
		},
		"required":             []string{name},
		"additionalProperties": false,
	}
}

// Tools returns every tool, in name order. See AN-4.
func Tools() []Tool {
	out := make([]Tool, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// tool returns the tool of a name.
func tool(name string) (Tool, bool) {
	for _, t := range registry {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// registry holds every tool of the server. Every tool except scaffold_slice
// reads and writes nothing. See the SDD, S16.
var registry = []Tool{
	{
		Name:        "list_modules",
		Description: "The contribution and the description of every module of the application.",
		InputSchema: noArguments(),
		run: func(ctx context.Context, s *Server, _ json.RawMessage) (any, error) {
			return document(ctx, s.Dir, inspect.Modules)
		},
	},
	{
		Name:        "describe_module",
		Description: "One module of the application, with its routes, its models and its events.",
		InputSchema: oneString("name", "The name of the module, such as posts."),
		run: func(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
			name, err := stringArg(args, "name")
			if err != nil {
				return nil, err
			}
			doc, err := document(ctx, s.Dir, inspect.Modules)
			if err != nil {
				return nil, err
			}
			rows, _ := doc["modules"].([]any)
			for _, raw := range rows {
				row, _ := raw.(map[string]any)
				if row["module"] == name {
					return row, nil
				}
			}
			return nil, fmt.Errorf("the application holds no module %q\n  → Run list_modules to see every module", name)
		},
	},
	{
		Name:        "list_routes",
		Description: "Every route of the application, with its method, its pattern, its handler and its middleware.",
		InputSchema: noArguments(),
		run: func(ctx context.Context, s *Server, _ json.RawMessage) (any, error) {
			return document(ctx, s.Dir, inspect.Routes)
		},
	},
	{
		Name:        "describe_model",
		Description: "One model of the application, with its table and its fields.",
		InputSchema: oneString("name", "The name of the model, such as Post."),
		run: func(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
			name, err := stringArg(args, "name")
			if err != nil {
				return nil, err
			}
			doc, err := document(ctx, s.Dir, inspect.Schema)
			if err != nil {
				return nil, err
			}
			rows, _ := doc["models"].([]any)
			for _, raw := range rows {
				row, _ := raw.(map[string]any)
				if strings.EqualFold(fmt.Sprint(row["name"]), name) {
					return row, nil
				}
			}
			return nil, fmt.Errorf("the application holds no model %q\n  → Run describe_module to see the models of one module", name)
		},
	},
	{
		Name:        "scaffold_slice",
		Description: "Write a new feature slice from the templates of the CLI, and register it in wire.go.",
		InputSchema: oneString("name", "The name of the feature, such as comment."),
		run: func(_ context.Context, s *Server, args json.RawMessage) (any, error) {
			name, err := stringArg(args, "name")
			if err != nil {
				return nil, err
			}
			if s.Slice == nil {
				return nil, errors.New("the server holds no scaffolder\n  → Run the server with `avero mcp`")
			}
			written, registered, err := s.Slice(s.Dir, name)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"slice":      strings.TrimSuffix(strings.ToLower(name), "s") + "s",
				"files":      written,
				"registered": registered,
				"next":       "Run run_verify. Apply the migration with `avero migrate up`.",
			}, nil
		},
	},
	{
		Name:        "run_verify",
		Description: "Run the gate of the application and return one record for each step.",
		InputSchema: noArguments(),
		run: func(ctx context.Context, s *Server, _ json.RawMessage) (any, error) {
			rep := verify.Run(ctx, s.Dir, verify.Steps(s.generate()))
			body, err := json.Marshal(rep)
			if err != nil {
				return nil, errors.New("the report does not encode as JSON")
			}
			var doc map[string]any
			if err := json.Unmarshal(body, &doc); err != nil {
				return nil, errors.New("the report does not parse")
			}
			return doc, nil
		},
	},
	{
		Name:        "explain_error",
		Description: "Map a build fault or a run-time fault to the position in the code of the person and to the repair.",
		InputSchema: oneString("message", "The output of the compiler, of the gate or of the application."),
		run: func(_ context.Context, s *Server, args json.RawMessage) (any, error) {
			message, err := stringArg(args, "message")
			if err != nil {
				return nil, err
			}
			return Explain(s.Dir, message), nil
		},
	},
}

// document runs one inspection and returns the JSON document.
func document(ctx context.Context, dir, command string) (map[string]any, error) {
	result, err := inspect.Run(ctx, dir, command, true)
	if err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("the application did not answer %s: %s", command, result.Stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &doc); err != nil {
		return nil, fmt.Errorf("the answer of %s does not parse as JSON", command)
	}
	return doc, nil
}

// stringArg reads one string argument.
func stringArg(raw json.RawMessage, name string) (string, error) {
	var args map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", errors.New("the arguments do not parse as JSON")
		}
	}
	value, _ := args[name].(string)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("the argument %q is absent\n  → Send %s as a string", name, name)
	}
	return value, nil
}
