package preflight

import (
	"os"
	"path/filepath"
)

// The load rules below follow the official docs listed in docs/sources.md.
// Where the docs do not say, the status is Unclear rather than a guess.

func nonEmpty(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular() && st.Size() > 0
}

func loadsFor(f found, ctx *walkContext) []Load {
	if f.scope == "global" {
		return globalLoads(f)
	}
	d := f.dir
	atStart := d == ctx.start
	inProject := ctx.projectRoot != "" && isWithin(d, ctx.projectRoot) && isWithin(ctx.start, d)
	atRoot := ctx.projectRoot != "" && d == ctx.projectRoot
	aboveProject := ctx.projectRoot != "" && !isWithin(d, ctx.projectRoot)

	var out []Load
	add := func(tool, status, note string) { out = append(out, Load{tool, status, note}) }

	switch f.kind {
	case KindClaudeMD, KindClaudeLocalMD:
		add(ToolClaude, Loads, "Claude Code loads these from the start folder and every folder above it at launch")
		if f.kind == KindClaudeMD && filepath.Base(f.path) == "CLAUDE.md" && filepath.Base(filepath.Dir(f.path)) != ".claude" {
			if atRoot || (ctx.projectRoot == "" && atStart) {
				add(ToolCopilot, Conditional, "Copilot can use a single CLAUDE.md at the repository root in place of AGENTS.md")
			}
		}

	case KindAgents:
		claudeNote := "default setting (Claude Code v2.1.277 or later): read only when no CLAUDE.md, .claude/CLAUDE.md or CLAUDE.local.md exists in the start folder or above"
		if ctx.claudeMDSet {
			add(ToolClaude, NotLoaded, "a CLAUDE.md file exists in this walk, so under the default setting Claude Code reads that instead")
		} else {
			add(ToolClaude, Loads, claudeNote)
		}
		if filepath.Base(filepath.Dir(f.path)) == ".claude" {
			break // .claude/AGENTS.md is a Claude Code location only
		}
		override := filepath.Join(d, "AGENTS.override.md")
		switch {
		case ctx.projectRoot == "" && !atStart:
			add(ToolCodex, NotLoaded, "no project root (.git) found, so Codex only checks the start folder")
		case aboveProject:
			add(ToolCodex, NotLoaded, "above the project root; Codex walks from the project root down to the start folder")
		case nonEmpty(override):
			add(ToolCodex, NotLoaded, "AGENTS.override.md in the same folder wins")
		default:
			add(ToolCodex, Loads, "Codex reads one instruction file per folder from the project root down to the start folder")
		}
		if atStart {
			add(ToolCursor, Loads, "AGENTS.md in the project root")
		} else {
			add(ToolCursor, Unclear, "Cursor documents AGENTS.md in the project and its subfolders, not in parent folders")
		}
		if inProject {
			add(ToolCopilot, Conditional, "Copilot uses the nearest AGENTS.md in the repository")
		} else {
			add(ToolCopilot, Unclear, "GitHub documents AGENTS.md inside the repository only")
		}
		add(ToolGemini, Conditional, "only if context.fileName in Gemini settings lists AGENTS.md")

	case KindAgentsOverride:
		add(ToolClaude, NotLoaded, "Claude Code docs say AGENTS.override.md is not read")
		switch {
		case ctx.projectRoot == "" && !atStart:
			add(ToolCodex, NotLoaded, "no project root (.git) found, so Codex only checks the start folder")
		case aboveProject:
			add(ToolCodex, NotLoaded, "above the project root")
		case !nonEmpty(f.path):
			add(ToolCodex, NotLoaded, "empty files are skipped")
		default:
			add(ToolCodex, Loads, "takes priority over AGENTS.md in the same folder")
		}

	case KindClaudeRule:
		if atStart {
			add(ToolClaude, Loads, "rules without paths frontmatter load at launch; rules with paths load when matching files are opened")
		} else {
			add(ToolClaude, Unclear, "docs describe .claude/rules in the project, not in parent folders")
		}

	case KindGeminiMD:
		add(ToolGemini, Loads, "Gemini CLI searches the workspace folders and their parents; the docs do not say where the parent search stops")
		if atRoot || (ctx.projectRoot == "" && atStart) {
			add(ToolCopilot, Conditional, "Copilot can use a single GEMINI.md at the repository root in place of AGENTS.md")
		}

	case KindCursorRules:
		add(ToolCursor, Unclear, ".cursorrules is not in Cursor's current rules docs, so whether it still loads is unclear")

	case KindCursorRule:
		if atStart {
			add(ToolCursor, Conditional, "project rule; applied always, by file pattern, by relevance or by @-mention depending on its frontmatter")
		} else {
			add(ToolCursor, Unclear, "Cursor documents .cursor/rules in the project, not in parent folders")
		}

	case KindCopilot:
		if atRoot || (ctx.projectRoot == "" && atStart) {
			add(ToolCopilot, Loads, "repository-wide custom instructions")
		} else {
			add(ToolCopilot, Unclear, "GitHub documents this file at the repository root only")
		}

	case KindCopilotPath:
		if atRoot || (ctx.projectRoot == "" && atStart) {
			add(ToolCopilot, Conditional, "applies to files matching its applyTo pattern")
		} else {
			add(ToolCopilot, Unclear, "GitHub documents .github/instructions at the repository root only")
		}

	case KindMCPJSON:
		if atStart {
			add(ToolClaude, Conditional, "project-scope MCP servers; Claude Code asks for approval before first use in an interactive session")
			add(ToolVSCode, Conditional, "workspace-root .mcp.json is read when the experimental chat.mcp.workspaceRootConfig.enabled setting is on")
		} else {
			add(ToolClaude, Unclear, "docs say .mcp.json lives in the project root; they do not say whether parent folders are searched")
		}

	case KindClaudeSettings:
		if atStart {
			add(ToolClaude, Loads, "shared project settings, read from the start folder; hooks in it wait for workspace trust in interactive sessions but run in -p sessions")
		} else {
			add(ToolClaude, NotLoaded, "Claude Code reads .claude/settings.json from the start folder only")
		}

	case KindClaudeLocal:
		switch {
		case atStart:
			add(ToolClaude, Loads, "local project settings")
		case atRoot:
			add(ToolClaude, Loads, "when started in a subfolder of a git repository, Claude Code reads this file at the repository root")
		default:
			add(ToolClaude, NotLoaded, "not the start folder or the repository root")
		}

	case KindVSCodeMCP:
		if atStart {
			add(ToolVSCode, Loads, "workspace MCP configuration (VS Code now labels it deprecated in its guided flow but still supports it)")
		} else {
			add(ToolVSCode, Unclear, "VS Code documents .vscode/mcp.json in the workspace folder")
		}

	case KindCodexConfig, KindCodexHooks:
		switch {
		case inProject || (ctx.projectRoot == "" && atStart):
			add(ToolCodex, Conditional, "project config loads only when the project is trusted")
		default:
			add(ToolCodex, NotLoaded, "outside the project root to start folder path")
		}

	case KindGeminiSettings:
		if atStart || atRoot {
			add(ToolGemini, Loads, "workspace .gemini/settings.json; hooks run when their event fires")
		} else {
			add(ToolGemini, Unclear, "Gemini docs describe project .gemini/settings.json, not parent folders")
		}

	case KindCursorMCP:
		if atStart || atRoot {
			add(ToolCursor, Loads, "project MCP configuration")
		} else {
			add(ToolCursor, Unclear, "Cursor documents project MCP config in the project root")
		}

	case KindVSCodeTasks:
		if atStart || atRoot {
			add(ToolVSCode, Conditional, "tasks with runOptions.runOn=folderOpen run when the folder is opened")
		} else {
			add(ToolVSCode, Unclear, "VS Code documents .vscode/tasks.json in the workspace folder")
		}

	case KindInitJS, KindInitPS1:
		add(ToolClaude, Conditional, "only if a SessionStart (or similar) hook in settings points at this file")
		add(ToolGemini, Conditional, "only if a Gemini hook points at this file")
		add(ToolCursor, Conditional, "only if a Cursor rule or MCP entry points at this file")
		add(ToolVSCode, Conditional, "only if a folderOpen task points at this file")

	case KindDevcontainer:
		if atStart || atRoot {
			add(ToolVSCode, Conditional, "VS Code / Dev Containers runs lifecycle commands when the container is created or started")
		} else {
			add(ToolVSCode, Unclear, "devcontainer.json is documented at the repository .devcontainer path")
		}
	}
	return out
}

func globalLoads(f found) []Load {
	switch f.kind {
	case KindClaudeSettings:
		return []Load{{ToolClaude, Loads, "user settings, every project"}}
	case KindClaudeMD:
		return []Load{{ToolClaude, Loads, "user or managed instructions, every project"}}
	case KindClaudeRule:
		return []Load{{ToolClaude, Loads, "user rules, every project"}}
	case KindClaudeUserJSON:
		return []Load{{ToolClaude, Loads, "user and local scope MCP servers"}}
	case KindCodexConfig:
		return []Load{{ToolCodex, Loads, "user config, every project"}}
	case KindCodexHooks:
		return []Load{{ToolCodex, Loads, "user-level hooks do not depend on project trust"}}
	case KindAgentsOverride:
		if nonEmpty(f.path) {
			return []Load{{ToolCodex, Loads, "global override, wins over the global AGENTS.md"}}
		}
		return []Load{{ToolCodex, NotLoaded, "empty"}}
	case KindAgents:
		if nonEmpty(filepath.Join(filepath.Dir(f.path), "AGENTS.override.md")) {
			return []Load{{ToolCodex, NotLoaded, "AGENTS.override.md in the Codex home wins"}}
		}
		return []Load{{ToolCodex, Loads, "global instructions, every project"}}
	case KindGeminiMD:
		return []Load{{ToolGemini, Loads, "global context file"}}
	case KindGeminiSettings:
		return []Load{{ToolGemini, Loads, "user settings, every project"}}
	case KindCopilotMCPGlobal:
		return []Load{{ToolCopilot, Loads, "Copilot Global MCP servers (VS Code and other Copilot tools)"}}
	}
	return nil
}
