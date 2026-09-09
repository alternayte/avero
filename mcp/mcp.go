// Package mcp serves the agent surface of an application over stdio, S16.
//
// `avero mcp` speaks the Model Context Protocol: JSON-RPC 2.0 over the
// standard input and the standard output. The server holds seven tools. Every
// tool except scaffold_slice reads and writes nothing, and the server never
// runs a command that a caller names. It runs the fixed steps of the gate and
// the inspection of the application, and nothing else.
package mcp

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// The protocol that the server speaks.
const (
	// ProtocolVersion is the version of the Model Context Protocol.
	ProtocolVersion = "2025-06-18"
	// ServerName names the server in the handshake.
	ServerName = "avero"
	// JSONRPCVersion is the version of the transport.
	JSONRPCVersion = "2.0"
)

// SchemaID names the schema of the tool results. See mcp/schema.json.
const SchemaID = "https://avero.dev/schema/mcp-results/v1"

// Schema holds mcp/schema.json. The structured result of each tool follows the
// definition of its name. See AN-3.
//
//go:embed schema.json
var Schema []byte

// Version is the version of the server. The release build sets it.
var Version = "dev"

// Server answers the Model Context Protocol for one application.
//
// The two functions are fields, because the scaffolder and the generators live
// in packages that this one does not import. `avero mcp` supplies them. See
// design rule 3.
type Server struct {
	// Dir is the root of the application.
	Dir string
	// Slice writes one feature slice. scaffold_slice calls it, and the CLI
	// supplies the same function that `avero slice` runs.
	Slice func(dir, name string) (files []string, registered bool, err error)
	// Generate proves that every generated file of the application is
	// current. The gate step of run_verify calls it.
	Generate func(dir string) error
}

// generate returns the check of the generated files, or a check that holds.
func (s *Server) generate() func(dir string) error {
	if s.Generate == nil {
		return func(string) error { return nil }
	}
	return s.Generate
}

// request is one JSON-RPC request.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response is one JSON-RPC answer.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is one JSON-RPC fault.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// The JSON-RPC codes that the server sends.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

// Serve reads requests from in and writes answers to out. It returns when the
// caller closes the input or cancels the context.
//
// Each line holds one JSON document, which is the line form of the transport.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewScanner(in)
	reader.Buffer(make([]byte, 0, 64<<10), 8<<20)
	writer := json.NewEncoder(out)

	for reader.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := reader.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			if err := writer.Encode(response{
				JSONRPC: JSONRPCVersion,
				Error:   &rpcError{Code: codeParse, Message: "the request does not parse as JSON"},
			}); err != nil {
				return err
			}
			continue
		}
		answer, send := s.handle(ctx, req)
		if !send {
			continue
		}
		if err := writer.Encode(answer); err != nil {
			return err
		}
	}
	if err := reader.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// handle answers one request. It reports whether the answer reaches the
// caller, because a notification carries no identifier and takes no answer.
func (s *Server) handle(ctx context.Context, req request) (response, bool) {
	answer := response{JSONRPC: JSONRPCVersion, ID: req.ID}
	notification := len(req.ID) == 0

	switch req.Method {
	case "initialize":
		answer.Result = map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": ServerName, "version": Version},
		}
	case "notifications/initialized", "initialized":
		return answer, false
	case "ping":
		answer.Result = map[string]any{}
	case "tools/list":
		answer.Result = map[string]any{"tools": Tools()}
	case "tools/call":
		result, err := s.call(ctx, req.Params)
		if err != nil {
			answer.Error = &rpcError{Code: codeInvalidParams, Message: err.Error()}
			break
		}
		answer.Result = result
	default:
		if notification {
			return answer, false
		}
		answer.Error = &rpcError{
			Code:    codeMethodNotFound,
			Message: fmt.Sprintf("the method %q is not known", req.Method),
		}
	}
	if notification {
		return answer, false
	}
	return answer, true
}

// callParams holds the arguments of tools/call.
type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// call runs one tool and returns the result of the protocol.
func (s *Server) call(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var params callParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, errors.New("the parameters of tools/call do not parse")
		}
	}
	if params.Name == "" {
		return nil, errors.New("tools/call names no tool")
	}
	tool, ok := tool(params.Name)
	if !ok {
		return nil, fmt.Errorf("the tool %q is not known", params.Name)
	}

	structured, err := tool.run(ctx, s, params.Arguments)
	if err != nil {
		// A fault of the application is a result, not a transport fault, so
		// the caller reads the message and the repair.
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}, nil
	}
	body, err := json.MarshalIndent(structured, "", "  ")
	if err != nil {
		return nil, errors.New("the result does not encode as JSON")
	}
	return map[string]any{
		"content":           []map[string]any{{"type": "text", "text": string(body)}},
		"structuredContent": structured,
		"isError":           false,
	}, nil
}
