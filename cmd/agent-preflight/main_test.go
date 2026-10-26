package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// localTree makes a fake home inside the package folder rather than the
// system temp folder, because agent-preflight flags anything started in a
// temp folder.
func localTree(t *testing.T) (home, start string) {
	t.Helper()
	home, err := os.MkdirTemp(".", "testhome-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	home, _ = filepath.Abs(home)
	start = filepath.Join(home, "proj")
	if err := os.MkdirAll(filepath.Join(start, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home, start
}

func runArgs(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	for _, k := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_BEDROCK_BASE_URL", "ANTHROPIC_VERTEX_BASE_URL",
		"OPENAI_BASE_URL", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		t.Setenv(k, "")
	}
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestExitCodes(t *testing.T) {
	home, start := localTree(t)
	state := filepath.Join(home, "state.json")

	code, out, _ := runArgs(t, "--home", home, "--state", state, start)
	if code != 0 || !strings.Contains(out, "RESULT: clean") {
		t.Fatalf("clean tree: code %d\n%s", code, out)
	}

	mcp := `{"mcpServers": {"tools": {"command": "npx", "args": ["-y", "x"]}}}`
	os.WriteFile(filepath.Join(start, ".mcp.json"), []byte(mcp), 0o644)
	code, out, _ = runArgs(t, "--home", home, "--state", state, start)
	if code != 1 || !strings.Contains(out, `MCP server "tools" is not in your approved baseline`) {
		t.Fatalf("new server: code %d\n%s", code, out)
	}

	code, _, errs := runArgs(t, "--home", home, "--state", state, "--approve", start)
	if code != 0 || !strings.Contains(errs, "MCP server tools") {
		t.Fatalf("approve: code %d\n%s", code, errs)
	}
	code, out, _ = runArgs(t, "--home", home, "--state", state, start)
	if code != 0 {
		t.Fatalf("after approve: code %d\n%s", code, out)
	}

	code, _, _ = runArgs(t, "--home", home, "--state", state, filepath.Join(home, "missing"))
	if code != 2 {
		t.Errorf("missing folder: code %d, want 2", code)
	}
	code, _, _ = runArgs(t, "--no-such-flag")
	if code != 2 {
		t.Errorf("bad flag: code %d, want 2", code)
	}
	code, _, _ = runArgs(t, "a", "b")
	if code != 2 {
		t.Errorf("two folders: code %d, want 2", code)
	}
}

func TestJSON(t *testing.T) {
	home, start := localTree(t)
	os.WriteFile(filepath.Join(start, "AGENTS.md"), []byte("Run npm install -g thing first.\n"), 0o644)
	code, out, _ := runArgs(t, "--json", "--home", home, "--state", filepath.Join(home, "s.json"), start)
	if code != 1 {
		t.Fatalf("code %d", code)
	}
	var r struct {
		StartDir string `json:"start_dir"`
		Files    []struct {
			Path string `json:"path"`
		} `json:"files"`
		Findings []struct {
			Category string `json:"category"`
			Line     int    `json:"line"`
		} `json:"findings"`
		Summary struct {
			Medium int `json:"medium"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if r.StartDir != start || len(r.Files) != 1 || len(r.Findings) != 1 ||
		r.Findings[0].Category != "install-fetch" || r.Findings[0].Line != 1 || r.Summary.Medium != 1 {
		t.Errorf("unexpected report: %+v", r)
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := runArgs(t, "--version")
	if code != 0 || !strings.HasPrefix(out, "agent-preflight ") {
		t.Errorf("code %d out %q", code, out)
	}
}
