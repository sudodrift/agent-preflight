package preflight

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Environment variables that send agent traffic somewhere else. Listed in the
// Claude Code environment variable reference and the Codex config reference.
var baseURLEnvKeys = []string{
	"ANTHROPIC_BASE_URL", "ANTHROPIC_BEDROCK_BASE_URL", "ANTHROPIC_VERTEX_BASE_URL",
	"OPENAI_BASE_URL",
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy",
}

// Environment variables that move where an agent reads its config from.
var configDirEnvKeys = []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME"}

func isBaseURLKey(k string) bool {
	for _, b := range baseURLEnvKeys {
		if k == b {
			return true
		}
	}
	return false
}

// configResult is what one config file adds to the report.
type configResult struct {
	servers  []MCPServer
	settings []Setting
	findings []Finding
}

func lineOf(data, needle string) int {
	i := strings.Index(data, needle)
	if i < 0 {
		return 0
	}
	return strings.Count(data[:i], "\n") + 1
}

func short(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asStrings(v any) []string {
	arr, _ := v.([]any)
	var out []string
	for _, a := range arr {
		out = append(out, fmt.Sprint(a))
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mcpFromMap reads {"name": {"command": ..., "args": [...], "url": ...}}.
func mcpFromMap(m map[string]any, source, data string) []MCPServer {
	var out []MCPServer
	for _, name := range sortedKeys(m) {
		def, _ := m[name].(map[string]any)
		s := MCPServer{Name: name, Source: source, Line: lineOf(data, `"`+name+`"`)}
		if def != nil {
			s.Command = asString(def["command"])
			s.Args = asStrings(def["args"])
			s.URL = asString(def["url"])
			if s.URL == "" {
				s.URL = asString(def["serverUrl"])
			}
		}
		out = append(out, s)
	}
	return out
}

// collectCommands finds every "command" string anywhere in a JSON value.
func collectCommands(v any, path string, out *[][2]string) {
	switch t := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(t) {
			if k == "command" {
				if s, ok := t[k].(string); ok {
					*out = append(*out, [2]string{path, s})
					continue
				}
			}
			collectCommands(t[k], strings.TrimPrefix(path+"."+k, "."), out)
		}
	case []any:
		for _, e := range t {
			collectCommands(e, path, out)
		}
	}
}

func parseJSONConfig(path, kind, data string, f *File, ctx *walkContext) configResult {
	var r configResult
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		r.findings = append(r.findings, Finding{Severity: Low, Category: "parse", Path: path,
			Message: "could not parse as JSON, so its contents were not checked", Detail: err.Error()})
		return r
	}
	switch kind {
	case KindMCPJSON, KindCopilotMCPGlobal:
		m, _ := root["mcpServers"].(map[string]any)
		r.servers = mcpFromMap(m, path, data)
	case KindVSCodeMCP:
		m, _ := root["servers"].(map[string]any)
		r.servers = mcpFromMap(m, path, data)
	case KindCursorMCP:
		// Cursor project MCP uses mcpServers; some armed repos also put command hooks here.
		m, _ := root["mcpServers"].(map[string]any)
		if m == nil {
			m, _ = root["servers"].(map[string]any)
		}
		r.servers = mcpFromMap(m, path, data)
		var cmds [][2]string
		collectCommands(root, "", &cmds)
		for _, c := range cmds {
			// Skip ones already reported as MCP server command fields.
			if strings.Contains(c[0], "mcpServers") || strings.Contains(c[0], "servers") {
				continue
			}
			r.findings = append(r.findings, Finding{Severity: High, Category: "auto-run", Path: path,
				Line: lineOf(data, c[1]), Message: "Cursor config runs a command automatically", Detail: short(c[0]+": "+c[1], 200)})
		}
	case KindClaudeUserJSON:
		m, _ := root["mcpServers"].(map[string]any)
		r.servers = mcpFromMap(m, path+" (user scope)", data)
		if projects, ok := root["projects"].(map[string]any); ok {
			for _, p := range sortedKeys(projects) {
				if p != ctx.start && p != ctx.projectRoot {
					continue
				}
				pm, _ := projects[p].(map[string]any)
				sm, _ := pm["mcpServers"].(map[string]any)
				r.servers = append(r.servers, mcpFromMap(sm, path+" (local scope for "+p+")", data)...)
			}
		}
	case KindClaudeSettings, KindClaudeLocal:
		r = claudeSettings(path, data, root)
	case KindGeminiSettings:
		r = geminiSettings(path, data, root)
	case KindVSCodeTasks:
		r = vscodeTasks(path, data, root)
	case KindDevcontainer:
		r = devcontainerSettings(path, data, root)
	case KindCodexHooks:
		var cmds [][2]string
		collectCommands(root, "", &cmds)
		for _, c := range cmds {
			r.findings = append(r.findings, Finding{Severity: High, Category: "auto-run", Path: path,
				Line: lineOf(data, c[1]), Message: "Codex hook runs a command automatically", Detail: short(c[0]+": "+c[1], 200)})
		}
	}
	return r
}

func claudeSettings(path, data string, root map[string]any) configResult {
	var r configResult
	if hooks, ok := root["hooks"].(map[string]any); ok {
		for _, event := range sortedKeys(hooks) {
			var cmds [][2]string
			collectCommands(hooks[event], event, &cmds)
			for _, c := range cmds {
				r.findings = append(r.findings, Finding{Severity: High, Category: "auto-run", Path: path,
					Line: lineOf(data, c[1]), Message: "hook runs a command automatically on " + event, Detail: short(c[1], 200)})
			}
		}
	}
	if sl, ok := root["statusLine"].(map[string]any); ok {
		if cmd := asString(sl["command"]); cmd != "" {
			r.findings = append(r.findings, Finding{Severity: Medium, Category: "auto-run", Path: path,
				Line: lineOf(data, `"statusLine"`), Message: "status line runs a command automatically", Detail: short(cmd, 200)})
		}
	}
	for _, k := range []string{"apiKeyHelper", "awsAuthRefresh", "awsCredentialExport", "otelHeadersHelper"} {
		if cmd := asString(root[k]); cmd != "" {
			r.findings = append(r.findings, Finding{Severity: Medium, Category: "auto-run", Path: path,
				Line: lineOf(data, `"`+k+`"`), Message: k + " runs a command to fetch credentials", Detail: short(cmd, 200)})
		}
	}
	if b, ok := root["enableAllProjectMcpServers"].(bool); ok && b {
		r.findings = append(r.findings, Finding{Severity: High, Category: "agent-config", Path: path,
			Line: lineOf(data, `"enableAllProjectMcpServers"`), Message: "enableAllProjectMcpServers is true, so every server in .mcp.json is approved without asking"})
	}
	if perms, ok := root["permissions"].(map[string]any); ok {
		if asString(perms["defaultMode"]) == "bypassPermissions" {
			r.findings = append(r.findings, Finding{Severity: High, Category: "agent-config", Path: path,
				Line: lineOf(data, `"defaultMode"`), Message: "permissions.defaultMode is bypassPermissions, so Claude Code will not ask before acting"})
		}
	}
	if env, ok := root["env"].(map[string]any); ok {
		for _, k := range sortedKeys(env) {
			if isBaseURLKey(k) {
				r.settings = append(r.settings, Setting{Key: k, Value: asString(env[k]), Source: path, Line: lineOf(data, `"`+k+`"`)})
			}
		}
	}
	if m, ok := root["mcpServers"].(map[string]any); ok {
		r.servers = append(r.servers, mcpFromMap(m, path, data)...)
	}
	return r
}

func parseCodexConfig(path, data string, project bool) configResult {
	var r configResult
	servers := map[string]*MCPServer{}
	var order []string
	ignored := ""
	if project {
		ignored = "Codex ignores this key in a project .codex/config.toml and prints a warning, but nothing legitimate should put it there"
	}
	for _, e := range parseTOML(data) {
		switch {
		case len(e.Table) >= 2 && e.Table[0] == "mcp_servers":
			name := e.Table[1]
			s, ok := servers[name]
			if !ok {
				s = &MCPServer{Name: name, Source: path, Line: e.Line}
				servers[name] = s
				order = append(order, name)
			}
			if len(e.Table) == 2 {
				switch e.Key {
				case "command":
					s.Command = e.Value.Str
				case "args":
					s.Args = e.Value.List
				case "url":
					s.URL = e.Value.Str
				}
			}
		case len(e.Table) == 0 && (e.Key == "openai_base_url" || e.Key == "chatgpt_base_url" || e.Key == "experimental_realtime_ws_base_url"):
			r.settings = append(r.settings, Setting{Key: e.Key, Value: e.Value.Str, Source: path, Line: e.Line, Note: ignored})
		case len(e.Table) == 2 && e.Table[0] == "model_providers" && e.Key == "base_url":
			r.settings = append(r.settings, Setting{Key: "model_providers." + e.Table[1] + ".base_url", Value: e.Value.Str, Source: path, Line: e.Line, Note: ignored})
		case len(e.Table) == 0 && e.Key == "notify":
			r.findings = append(r.findings, Finding{Severity: Medium, Category: "auto-run", Path: path, Line: e.Line,
				Message: "notify runs a command after each turn", Detail: short(strings.Join(e.Value.List, " "), 200)})
		case len(e.Table) >= 1 && e.Table[0] == "hooks" && e.Key == "command":
			r.findings = append(r.findings, Finding{Severity: High, Category: "auto-run", Path: path, Line: e.Line,
				Message: "Codex hook runs a command automatically", Detail: short(strings.Join(e.Table, ".")+": "+e.Value.Str+strings.Join(e.Value.List, " "), 200)})
		}
	}
	for _, n := range order {
		r.servers = append(r.servers, *servers[n])
	}
	return r
}


// geminiSettings reads Gemini CLI project/user settings.json hooks.
// Docs: https://geminicli.com/docs/hooks/ and https://geminicli.com/docs/reference/configuration/
func geminiSettings(path, data string, root map[string]any) configResult {
	var r configResult
	if hooks, ok := root["hooks"].(map[string]any); ok {
		for _, event := range sortedKeys(hooks) {
			var cmds [][2]string
			collectCommands(hooks[event], event, &cmds)
			for _, c := range cmds {
				r.findings = append(r.findings, Finding{Severity: High, Category: "auto-run", Path: path,
					Line: lineOf(data, c[1]), Message: "Gemini hook runs a command automatically on " + event, Detail: short(c[1], 200)})
			}
		}
	}
	return r
}

// vscodeTasks flags tasks that auto-run when a folder is opened.
// Docs: https://code.visualstudio.com/docs/editor/tasks#_run-behavior (runOptions.runOn)
func vscodeTasks(path, data string, root map[string]any) configResult {
	var r configResult
	tasks, _ := root["tasks"].([]any)
	for _, t := range tasks {
		tm, _ := t.(map[string]any)
		if tm == nil {
			continue
		}
		runOpts, _ := tm["runOptions"].(map[string]any)
		runOn := ""
		if runOpts != nil {
			runOn = asString(runOpts["runOn"])
		}
		if runOn != "folderOpen" {
			continue
		}
		label := asString(tm["label"])
		cmd := asString(tm["command"])
		if cmd == "" {
			if args := asStrings(tm["args"]); len(args) > 0 {
				cmd = strings.Join(args, " ")
			}
		}
		detail := short(label+": "+cmd, 200)
		r.findings = append(r.findings, Finding{Severity: High, Category: "auto-run", Path: path,
			Line: lineOf(data, `"runOn"`), Message: "VS Code task runs automatically on folderOpen", Detail: detail})
	}
	return r
}

// devcontainerSettings flags lifecycle commands that run when a container is created or started.
// Docs: https://containers.dev/implementors/json_reference/
func devcontainerSettings(path, data string, root map[string]any) configResult {
	var r configResult
	for _, k := range []string{
		"postCreateCommand", "postStartCommand", "postAttachCommand",
		"initializeCommand", "onCreateCommand", "updateContentCommand",
	} {
		if v, ok := root[k]; ok && v != nil {
			detail := short(fmt.Sprint(v), 200)
			if detail == "" || detail == "map[]" || detail == "[]" {
				continue
			}
			r.findings = append(r.findings, Finding{Severity: Medium, Category: "auto-run", Path: path,
				Line: lineOf(data, `"`+k+`"`), Message: "devcontainer " + k + " runs a command automatically", Detail: detail})
		}
	}
	return r
}
