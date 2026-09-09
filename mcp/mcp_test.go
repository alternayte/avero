package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alternayte/avero/mcp"
)

// session drives one server over a pipe of lines.
type session struct {
	t      *testing.T
	server *mcp.Server
	id     int
}

// newSession returns a session on one directory.
func newSession(t *testing.T, server *mcp.Server) *session {
	t.Helper()
	return &session{t: t, server: server}
}

// send writes one request and returns the answer.
func (s *session) send(method string, params any) map[string]any {
	s.t.Helper()
	s.id++
	body := map[string]any{"jsonrpc": "2.0", "id": s.id, "method": method}
	if params != nil {
		body["params"] = params
	}
	line, err := json.Marshal(body)
	if err != nil {
		s.t.Fatalf("Marshal returned %v", err)
	}
	var out strings.Builder
	if err := s.server.Serve(context.Background(), strings.NewReader(string(line)+"\n"), &out); err != nil {
		s.t.Fatalf("Serve returned %v", err)
	}
	var answer map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &answer); err != nil {
		s.t.Fatalf("the answer does not parse: %v\n%s", err, out.String())
	}
	return answer
}

// call runs one tool and returns its structured result.
func (s *session) call(name string, args map[string]any) (map[string]any, bool) {
	s.t.Helper()
	answer := s.send("tools/call", map[string]any{"name": name, "arguments": args})
	result, ok := answer["result"].(map[string]any)
	if !ok {
		s.t.Fatalf("the answer holds no result: %v", answer)
	}
	if failed, _ := result["isError"].(bool); failed {
		text := ""
		if content, ok := result["content"].([]any); ok && len(content) > 0 {
			if row, ok := content[0].(map[string]any); ok {
				text = fmt.Sprint(row["text"])
			}
		}
		return map[string]any{"text": text}, false
	}
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		s.t.Fatalf("the result holds no structured content: %v", result)
	}
	return structured, true
}

func TestTheHandshakeStatesTheProtocolAndTheTools(t *testing.T) {
	s := newSession(t, &mcp.Server{Dir: t.TempDir()})
	answer := s.send("initialize", map[string]any{"protocolVersion": mcp.ProtocolVersion})
	result, ok := answer["result"].(map[string]any)
	if !ok {
		t.Fatalf("the answer holds no result: %v", answer)
	}
	if result["protocolVersion"] != mcp.ProtocolVersion {
		t.Fatalf("the version is %v, want %s", result["protocolVersion"], mcp.ProtocolVersion)
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != mcp.ServerName {
		t.Fatalf("the server names itself %v", info["name"])
	}

	list := s.send("tools/list", nil)
	tools, _ := list["result"].(map[string]any)["tools"].([]any)
	names := make([]string, 0, len(tools))
	for _, raw := range tools {
		row, _ := raw.(map[string]any)
		names = append(names, fmt.Sprint(row["name"]))
		if row["description"] == "" || row["inputSchema"] == nil {
			t.Fatalf("the tool %v states no description or no schema", row["name"])
		}
	}
	want := "describe_model,describe_module,explain_error,list_modules,list_routes,run_verify,scaffold_slice"
	if strings.Join(names, ",") != want {
		t.Fatalf("the tools are %v, want %s in name order", names, want)
	}
}

func TestAnUnknownMethodAndAnUnknownToolStateTheFault(t *testing.T) {
	s := newSession(t, &mcp.Server{Dir: t.TempDir()})
	answer := s.send("resources/list", nil)
	fault, ok := answer["error"].(map[string]any)
	if !ok || !strings.Contains(fmt.Sprint(fault["message"]), "resources/list") {
		t.Fatalf("the answer is %v", answer)
	}

	answer = s.send("tools/call", map[string]any{"name": "delete_everything"})
	fault, ok = answer["error"].(map[string]any)
	if !ok || !strings.Contains(fmt.Sprint(fault["message"]), "delete_everything") {
		t.Fatalf("the answer is %v", answer)
	}
}

func TestANotificationTakesNoAnswer(t *testing.T) {
	server := &mcp.Server{Dir: t.TempDir()}
	var out strings.Builder
	line := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	if err := server.Serve(context.Background(), strings.NewReader(line), &out); err != nil {
		t.Fatalf("Serve returned %v", err)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Fatalf("the server answered a notification: %q", out.String())
	}
}

func TestATextThatIsNotJSONStatesTheFault(t *testing.T) {
	server := &mcp.Server{Dir: t.TempDir()}
	var out strings.Builder
	if err := server.Serve(context.Background(), strings.NewReader("not json\n"), &out); err != nil {
		t.Fatalf("Serve returned %v", err)
	}
	if !strings.Contains(out.String(), "does not parse") {
		t.Fatalf("the server answered %q", out.String())
	}
}

func TestAToolFaultReachesTheCallerAsAResult(t *testing.T) {
	// A fault of the application is a result, not a transport fault, so the
	// caller reads the message and the repair.
	s := newSession(t, &mcp.Server{Dir: t.TempDir()})
	result, ok := s.call("list_modules", nil)
	if ok {
		t.Fatalf("the tool answered %v for a directory that holds no application", result)
	}
	if !strings.Contains(fmt.Sprint(result["text"]), "avero new") {
		t.Fatalf("the fault states no repair: %v", result)
	}
}

func TestAnAbsentArgumentStatesTheRepair(t *testing.T) {
	s := newSession(t, &mcp.Server{Dir: t.TempDir()})
	result, ok := s.call("describe_module", map[string]any{})
	if ok {
		t.Fatalf("the tool answered %v with no name", result)
	}
	if !strings.Contains(fmt.Sprint(result["text"]), "name") {
		t.Fatalf("the fault is %v", result)
	}
}

func TestScaffoldSliceNeedsTheScaffolderOfTheCommand(t *testing.T) {
	s := newSession(t, &mcp.Server{Dir: t.TempDir()})
	result, ok := s.call("scaffold_slice", map[string]any{"name": "comment"})
	if ok {
		t.Fatalf("the tool wrote a slice with no scaffolder: %v", result)
	}
	if !strings.Contains(fmt.Sprint(result["text"]), "avero mcp") {
		t.Fatalf("the fault is %v", result)
	}
}

func TestTheSchemaHoldsADefinitionForEachTool(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(mcp.Schema, &schema); err != nil {
		t.Fatalf("mcp/schema.json does not parse: %v", err)
	}
	if schema["$id"] != mcp.SchemaID {
		t.Fatalf("the schema names itself %v, want %s", schema["$id"], mcp.SchemaID)
	}
	defs, _ := schema["$defs"].(map[string]any)
	for _, tool := range mcp.Tools() {
		if _, ok := defs[tool.Name]; !ok {
			t.Fatalf("mcp/schema.json holds no definition of %s", tool.Name)
		}
	}
	if len(defs) != len(mcp.Tools()) {
		t.Fatalf("the schema holds %d definitions and the server holds %d tools", len(defs), len(mcp.Tools()))
	}
}
