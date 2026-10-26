package preflight

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Tool names used in Load entries.
const (
	ToolClaude  = "Claude Code"
	ToolCodex   = "Codex"
	ToolCursor  = "Cursor"
	ToolGemini  = "Gemini CLI"
	ToolCopilot = "GitHub Copilot"
	ToolVSCode  = "VS Code"
)

// File kinds.
const (
	KindAgents           = "AGENTS.md"
	KindAgentsOverride   = "AGENTS.override.md"
	KindClaudeMD         = "CLAUDE.md"
	KindClaudeLocalMD    = "CLAUDE.local.md"
	KindClaudeRule       = "Claude rule"
	KindGeminiMD         = "GEMINI.md"
	KindCursorRules      = ".cursorrules"
	KindCursorRule       = "Cursor rule"
	KindCopilot          = "copilot-instructions.md"
	KindCopilotPath      = "Copilot path instructions"
	KindMCPJSON          = ".mcp.json"
	KindClaudeSettings   = "Claude settings"
	KindClaudeLocal      = "Claude local settings"
	KindClaudeUserJSON   = "Claude user config (.claude.json)"
	KindVSCodeMCP        = "VS Code mcp.json"
	KindCodexConfig      = "Codex config.toml"
	KindCodexHooks       = "Codex hooks.json"
	KindCopilotMCPGlobal = "Copilot global mcp-config.json"
)

// isInstruction reports whether a kind is a markdown instruction file.
func isInstruction(kind string) bool {
	switch kind {
	case KindAgents, KindAgentsOverride, KindClaudeMD, KindClaudeLocalMD, KindClaudeRule,
		KindGeminiMD, KindCursorRules, KindCursorRule, KindCopilot, KindCopilotPath:
		return true
	}
	return false
}

type candidate struct {
	rel  string
	kind string
}

var perDir = []candidate{
	{"AGENTS.md", KindAgents},
	{"AGENTS.override.md", KindAgentsOverride},
	{"CLAUDE.md", KindClaudeMD},
	{".claude/CLAUDE.md", KindClaudeMD},
	{"CLAUDE.local.md", KindClaudeLocalMD},
	{".claude/AGENTS.md", KindAgents},
	{"GEMINI.md", KindGeminiMD},
	{".cursorrules", KindCursorRules},
	{".github/copilot-instructions.md", KindCopilot},
	{".mcp.json", KindMCPJSON},
	{".claude/settings.json", KindClaudeSettings},
	{".claude/settings.local.json", KindClaudeLocal},
	{".vscode/mcp.json", KindVSCodeMCP},
	{".codex/config.toml", KindCodexConfig},
	{".codex/hooks.json", KindCodexHooks},
}

// perDirTrees are folders whose files are all rules of one kind.
var perDirTrees = []struct {
	rel    string
	suffix string
	kind   string
}{
	{".cursor/rules", ".mdc", KindCursorRule},
	{".claude/rules", ".md", KindClaudeRule},
	{".github/instructions", ".instructions.md", KindCopilotPath},
}

// walkContext holds what the load rules need to know.
type walkContext struct {
	start       string
	home        string
	projectRoot string // nearest folder at or above start with .git, "" if none
	dirs        []string
	claudeMDSet bool // a CLAUDE.md, .claude/CLAUDE.md or CLAUDE.local.md exists in the walk
}

// ancestors returns start and its parents up to and including stop. If stop
// is not an ancestor of start, it goes up to the filesystem root.
func ancestors(start, stop string) []string {
	var out []string
	d := start
	for {
		out = append(out, d)
		if d == stop {
			return out
		}
		p := filepath.Dir(d)
		if p == d {
			return out
		}
		d = p
	}
}

func findProjectRoot(start string) string {
	d := start
	for {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			return ""
		}
		d = p
	}
}

// isWithin reports whether dir is inside (or equal to) root.
func isWithin(dir, root string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(root, dir)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func regularFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// treeFiles lists files under dir ending in suffix, sorted. Symlinked
// folders are not followed.
func treeFiles(dir, suffix string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), suffix) && regularFile(p) {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

type found struct {
	path  string
	kind  string
	dir   string // the folder in the walk this belongs to, "" for global
	scope string
}

// discover finds every candidate file in the walk and in global config.
func discover(ctx *walkContext, env func(string) string) []found {
	var out []found
	seen := map[string]bool{}
	add := func(f found) {
		if seen[f.path] {
			return
		}
		seen[f.path] = true
		out = append(out, f)
	}
	// Global files first, so that ~/.claude/settings.json and friends are
	// treated as user config even though the walk also passes through home.
	for _, g := range globalCandidates(ctx.home, env) {
		if regularFile(g.path) {
			add(g)
		}
		if g.kind == KindClaudeRule {
			for _, p := range treeFiles(g.path, ".md") {
				add(found{p, KindClaudeRule, "", "global"})
			}
		}
	}
	for _, d := range ctx.dirs {
		scope := "parent"
		if d == ctx.start {
			scope = "start"
		}
		for _, c := range perDir {
			p := filepath.Join(d, c.rel)
			if regularFile(p) {
				add(found{p, c.kind, d, scope})
			}
		}
		for _, t := range perDirTrees {
			for _, p := range treeFiles(filepath.Join(d, t.rel), t.suffix) {
				add(found{p, t.kind, d, scope})
			}
		}
	}
	return out
}

func globalCandidates(home string, env func(string) string) []found {
	claudeDir := filepath.Join(home, ".claude")
	if v := env("CLAUDE_CONFIG_DIR"); v != "" {
		claudeDir = v
	}
	codexHome := filepath.Join(home, ".codex")
	if v := env("CODEX_HOME"); v != "" {
		codexHome = v
	}
	g := func(p, kind string) found { return found{p, kind, "", "global"} }
	return []found{
		g(filepath.Join(claudeDir, "settings.json"), KindClaudeSettings),
		g(filepath.Join(claudeDir, "CLAUDE.md"), KindClaudeMD),
		g(filepath.Join(claudeDir, "rules"), KindClaudeRule),
		g(filepath.Join(home, ".claude.json"), KindClaudeUserJSON),
		g(filepath.Join(codexHome, "config.toml"), KindCodexConfig),
		g(filepath.Join(codexHome, "AGENTS.override.md"), KindAgentsOverride),
		g(filepath.Join(codexHome, "AGENTS.md"), KindAgents),
		g(filepath.Join(codexHome, "hooks.json"), KindCodexHooks),
		g(filepath.Join(home, ".gemini", "GEMINI.md"), KindGeminiMD),
		g(filepath.Join(home, ".copilot", "mcp-config.json"), KindCopilotMCPGlobal),
		g("/etc/claude-code/CLAUDE.md", KindClaudeMD),
		g("/Library/Application Support/ClaudeCode/CLAUDE.md", KindClaudeMD),
	}
}
