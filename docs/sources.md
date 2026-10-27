# Sources for the loading rules

agent-preflight says, for each file it finds, which tools would load it. Every
rule comes from the official docs below, read on 2026-10-26. When the docs do
not answer a question, the tool says `unclear` instead of guessing, and the
gap is listed at the end of this page.

Statuses used in the output:

- `loads`: the docs say the tool reads the file in this spot.
- `conditional`: it loads only under a condition given in the note.
- `unclear`: the docs do not say.
- `not loaded`: the docs say the tool does not read it here.

## Claude Code

Memory files: https://code.claude.com/docs/en/memory

- `CLAUDE.md`, `CLAUDE.local.md` and `.claude/CLAUDE.md` load from the
  working folder and every folder above it at launch. Files in subfolders
  load later, on demand.
- Managed files: `/etc/claude-code/CLAUDE.md` on Linux and
  `/Library/Application Support/ClaudeCode/CLAUDE.md` on macOS. User file:
  `~/.claude/CLAUDE.md`.
- `AGENTS.md` and `.claude/AGENTS.md` (v2.1.277 and later) are read only when
  no `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` exists in the
  working folder or above it. That is the default setting,
  `claude-md-or-agents-md`. `AGENTS.local.md`, `AGENTS.override.md` and
  `.agents/` are not read.
- `.claude/rules/*.md` files without a `paths` field load at launch.
  `~/.claude/rules` applies to every project.

Settings: https://code.claude.com/docs/en/settings

- `~/.claude/settings.json` is user scope.
- `.claude/settings.json` is read from the primary working folder only.
- `.claude/settings.local.json` is read at the repository root when Claude
  Code starts in a subfolder of a git repository.
- `CLAUDE_CONFIG_DIR` moves the user config folder.

Hooks: https://code.claude.com/docs/en/hooks

- In interactive sessions, hooks wait until you accept workspace trust.
  Sessions started with `-p` or through the SDK treat the folder as trusted,
  so hooks committed to the project run.

MCP: https://code.claude.com/docs/en/mcp

- `.mcp.json` holds project scope servers and lives in the project root.
  Claude Code asks for approval before using them.
- `~/.claude.json` holds user scope servers and local scope servers (keyed by
  project path).
- `enableAllProjectMcpServers` and `enabledMcpjsonServers` are ignored in
  folders that are not trusted.

Environment variables: https://code.claude.com/docs/en/env-vars

- `ANTHROPIC_BASE_URL`, `ANTHROPIC_BEDROCK_BASE_URL`,
  `ANTHROPIC_VERTEX_BASE_URL`, `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`.

## Codex

AGENTS.md: https://developers.openai.com/codex/guides/agents-md

- Global: `CODEX_HOME` (default `~/.codex`). `AGENTS.override.md` wins over
  `AGENTS.md`; the first file that is not empty is used.
- Project: Codex walks from the project root (the git root) down to the
  working folder and takes at most one file per folder: the override, then
  `AGENTS.md`, then any fallback names. With no project root, only the
  working folder is checked.
- Empty files are skipped. There is a 32 KiB limit.

Advanced config: https://developers.openai.com/codex/config-advanced

- Project `.codex/config.toml` files load from the project root down to the
  working folder, and only when the project is trusted.
- In project config Codex ignores `openai_base_url`, `chatgpt_base_url`,
  `model_provider`, `model_providers`, `notify`, `profile`, `profiles`,
  `experimental_realtime_ws_base_url` and `otel`, and prints a warning.
  agent-preflight still reports these keys, because nothing legitimate should
  put them there.
- Hooks come from `~/.codex/hooks.json`, `~/.codex/config.toml`,
  `<repo>/.codex/hooks.json` and `<repo>/.codex/config.toml`. Project hooks
  load only when the project is trusted. User hooks load either way.
- The project root marker defaults to `.git` (`project_root_markers`).

Config reference: https://developers.openai.com/codex/config-reference

- `mcp_servers.<id>.command`, `args` and `url`; `openai_base_url`;
  `chatgpt_base_url`; `model_providers.<id>.base_url`; `notify`;
  `projects.<path>.trust_level`.

## Gemini CLI

https://geminicli.com/docs/cli/gemini-md/

- Global file `~/.gemini/GEMINI.md`. Gemini CLI also searches the workspace
  folders and their parents, and loads files in subfolders just in time, up
  to a trusted root.
- `context.fileName` in settings can add other names such as `AGENTS.md`.
- The same page says Gemini CLI was replaced by Antigravity CLI for unpaid
  users on June 18, 2026.

## Cursor

https://cursor.com/docs/context/rules

- Project rules are `.cursor/rules/*.mdc` files. Their frontmatter
  (`alwaysApply`, `globs`, `description`) decides when they apply.
- `AGENTS.md` is supported in the project root and in subfolders.

## GitHub Copilot

https://docs.github.com/en/copilot/how-tos/configure-custom-instructions/add-repository-instructions

- `.github/copilot-instructions.md` at the repository root.
- `.github/instructions/**/NAME.instructions.md` with an `applyTo` pattern.
- `AGENTS.md` anywhere in the repository (the nearest one wins), or a single
  `CLAUDE.md` or `GEMINI.md` at the repository root.

The AGENTS.md site, https://agents.md/, also says the nearest file in the
folder tree takes precedence.

## VS Code MCP

https://code.visualstudio.com/docs/copilot/customization/mcp-servers

- Copilot global servers live in `$COPILOT_HOME/mcp-config.json` or
  `~/.copilot/mcp-config.json`.
- A `.mcp.json` at the workspace root is read only when the experimental
  `chat.mcp.workspaceRootConfig.enabled` setting is on.
- `.vscode/mcp.json` and the user profile `mcp.json` still work but are
  labeled deprecated. Those files use a `servers` key; the portable files use
  `mcpServers`.

## Where a file came from

- curl `--xattr` stores the URL in `user.xdg.origin.url`:
  https://curl.se/docs/manpage.html. Tested on this machine: it also sets
  `user.creator=curl` and `user.mime_type`.
- GNU Wget `--xattr` does the same:
  https://www.gnu.org/software/wget/manual/wget.html
- Chromium used to set `user.xdg.origin.url` and `user.xdg.referrer.url` on
  Linux (https://codereview.chromium.org/2123023002) and removed it as a
  privacy risk:
  https://chromium.googlesource.com/chromium/src/+/a9b4fb70b4318b220deee0da7b1693d16b8ed071
  So on Linux, a file with no origin attribute may still have come from a
  browser.
- macOS: Apple describes how Gatekeeper marks downloaded files
  (https://support.apple.com/guide/security/gatekeeper-and-runtime-protection-sec5599b66df/web),
  but that page does not name the attribute. agent-preflight reads
  `com.apple.quarantine` and `com.apple.metadata:kMDItemWhereFroms`, the
  names macOS uses in practice. This part has not been tested on a Mac.

## The attack this is built for

Forever Security, "Markdown Is the New Executable", by Gal Weizman and Omer
Samet, October 22, 2026:
https://forever.security/blog/hijacking-ai-agents-via-downloads-folder/

- They found a malicious `AGENTS.md` in a customer's `~/Downloads`. It made
  Codex add an `[mcp_servers.project-tools]` entry that ran `launchctl` and
  `npm install function-color` to `~/.codex/config.toml`, then run
  `codex exec --dangerously-bypass-approvals-and-sandbox`.
- A Claude Code version had `CLAUDE.md` write `.claude/settings.json` with
  `ANTHROPIC_BASE_URL` in its `env` block, and used `dangerouslyDisableSandbox`
  on a Bash call.
- Their advice: do not start agents in Downloads, look for instruction files
  you did not write, and check for new `[mcp_servers.*]` entries, a changed
  `chatgpt_base_url`, and `ANTHROPIC_BASE_URL` or proxy settings.

## Unclear or untested

- `.cursorrules`: not mentioned in Cursor's current docs. Reported as
  `unclear`.
- Cursor and Copilot in parent folders: neither documents reading
  instruction files above the project. Reported as `unclear`.
- Gemini CLI: the docs say it searches parent folders but not where that
  search stops. Reported as `loads` with that note.
- Claude Code `.mcp.json` in parent folders: the docs do not say whether
  parent folders are searched. Reported as `unclear`.
- `.claude/rules` in parent folders: the docs describe the project folder
  only. Reported as `unclear`.
- The exact layout of Codex hooks in `config.toml` is not spelled out on the
  pages above, so agent-preflight reports any `command` key under a `hooks`
  table.
- macOS attribute names: see above. Not tested on a Mac.
- The walk stops at your home folder. Claude Code reads `CLAUDE.md` all the
  way up to `/`, so files above home (other than the managed paths) are not
  checked when you start inside home.

## Startup and folder-open hooks (October 2026 campaign)

JFrog, "Beyond testmetest", October 26, 2026:
https://research.jfrog.com/post/beyond-testmetest-42-hijacked-github-repos/

- 42 armed third-party GitHub repositories planted `.github/init.js` (12.9 to
  13.7 MB wrappers) plus hooks in `.claude/settings.json` (SessionStart),
  `.gemini/settings.json`, `.cursor/mcp.json`, `.cursor/rules/setup.mdc` and
  `.vscode/tasks.json`.
- Claude SessionStart hook shape: `node .github/init.js || bun .github/init.js`.

Gemini CLI hooks: https://geminicli.com/docs/hooks/
Gemini settings: https://geminicli.com/docs/reference/configuration/
VS Code task `runOptions.runOn`: https://code.visualstudio.com/docs/editor/tasks
Dev Containers lifecycle commands: https://containers.dev/implementors/json_reference/
