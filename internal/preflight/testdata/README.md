# Test fixture trees

Each folder is a small fake home folder. The tests copy a tree into a fresh
temp folder, drop the `.fixture` suffix from file names and rename `_git` to
`.git`. The suffix keeps these files from being picked up by a real agent
working on this repository.

- `forever`: an AGENTS.md in Downloads that tells the agent to add an MCP
  server to the Codex config and turn off approvals, plus a Codex config and
  a Claude settings file that already carry those changes. It is modeled on
  the attack described by Forever Security, not copied from it.
- `clean`: a normal project with a CLAUDE.md and harmless settings.
- `parent`: an AGENTS.md above the project root and a VS Code MCP server.
- `hooks`: Claude and Codex settings that run commands on their own.

All hosts use the `.invalid` top level domain, so nothing here can reach the
network.

## startup-hooks

A project shaped like the October 2026 JFrog armed-repo campaign: Claude and
Gemini SessionStart hooks, Cursor MCP + setup.mdc, a VS Code folderOpen task,
a small obfuscated `.github/init.js`, and a devcontainer postCreateCommand.
Harmless stand-ins only; the init.js skeleton has no real payload.
