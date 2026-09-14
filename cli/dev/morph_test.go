package dev_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheScrollPositionSurvivesAReload runs the reload client against a small
// document model and proves that a reload morphs the page.
//
// The page keeps its scroll offset and the identity of its elements, because
// the client fetches and morphs and never navigates. The harness runs under
// node, which is optional, so the test skips on a machine that holds none. See
// DX-9 and S15.
func TestTheScrollPositionSurvivesAReload(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is absent, and it is optional")
	}
	dir := t.TempDir()
	before := filepath.Join(dir, "before.html")
	after := filepath.Join(dir, "after.html")
	write(t, before, `<body><main id="main"><h1 id="title">Posts</h1><p>one</p></main></body>`)
	write(t, after, `<body><main id="main"><h1 id="title">Journal</h1><p>one</p></main></body>`)

	out, err := exec.Command(node, "testdata/morph_harness.js", before, after, "client.js").CombinedOutput()
	if err != nil {
		t.Fatalf("the harness failed: %v\n%s", err, out)
	}
	var result struct {
		ScrollY      int    `json:"scrollY"`
		KeptIdentity bool   `json:"keptIdentity"`
		Text         string `json:"text"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("the harness wrote %q: %v", out, err)
	}
	if result.ScrollY != 120 {
		t.Fatalf("the scroll offset is %d, want 120", result.ScrollY)
	}
	if !result.KeptIdentity {
		t.Fatal("the reload replaced the document, and a morph must keep it")
	}
	if !strings.Contains(result.Text, "Journal") {
		t.Fatalf("the page holds %q, want the new heading", result.Text)
	}
}

// write puts one file into a test directory.
func write(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
}
