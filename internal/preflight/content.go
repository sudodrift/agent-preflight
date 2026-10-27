package preflight

import (
	"bufio"
	"regexp"
	"strings"
)

type contentRule struct {
	severity string
	category string
	re       *regexp.Regexp
	message  string
}

// Rules for instruction files. They flag text that asks the agent to install
// or fetch things, change its own config, reroute its traffic, or turn off its
// safety checks. Several come straight from the payloads in the Forever
// Security write-up (see docs/sources.md).
var contentRules = []contentRule{
	{High, "agent-config", regexp.MustCompile(`(?i)--dangerously-bypass-approvals-and-sandbox|--dangerously-skip-permissions|dangerouslyDisableSandbox|bypassPermissions|--yolo\b`),
		"asks the agent to turn off its sandbox or approval prompts"},
	{High, "base-url", regexp.MustCompile(`(?i)\b(ANTHROPIC_BASE_URL|ANTHROPIC_BEDROCK_BASE_URL|ANTHROPIC_VERTEX_BASE_URL|OPENAI_BASE_URL|openai_base_url|chatgpt_base_url|HTTPS?_PROXY|ALL_PROXY)\b`),
		"mentions an API base URL or proxy setting"},
	{High, "agent-config", regexp.MustCompile(`(?i)(\.codex/config\.toml|\.claude/settings(\.local)?\.json|\.claude\.json|\.mcp\.json|\.vscode/mcp\.json|mcp_servers|mcpServers|mcp-config\.json)`),
		"mentions an agent config file or MCP server setup"},
	{High, "auto-run", regexp.MustCompile(`(?i)\.github/init\.(js|ps1)\b`),
		"mentions .github/init.js or init.ps1, a common startup-hook payload path"},
	{High, "install-fetch", regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|]*\|\s*(sudo\s+)?(ba|z|da)?sh\b`),
		"pipes a download straight into a shell"},
	{High, "auto-run", regexp.MustCompile(`(?i)\b(launchctl|crontab|schtasks|systemctl\s+--user)\b`),
		"mentions a tool that schedules or persists commands"},
	{Medium, "install-fetch", regexp.MustCompile(`(?i)\b((npm|pnpm|bun)\s+(i|install|add)\b|yarn\s+add\b|npx\s|pip3?\s+install\b|uv\s+(pip\s+install|add|tool\s+install)\b|uvx\s|pipx\s+install\b|brew\s+install\b|go\s+install\b|cargo\s+install\b|gem\s+install\b|apt(-get)?\s+install\b)`),
		"asks the agent to install something"},
	{Medium, "install-fetch", regexp.MustCompile(`(?i)\b(curl|wget|Invoke-WebRequest|iwr)\b|\bgit\s+clone\b`),
		"asks the agent to fetch something from the network"},
	{Medium, "auto-run", regexp.MustCompile(`(?i)\bbase64\s+(-d|--decode)\b|\b(ba|z)?sh\s+-l?c\b|\bpython3?\s+-c\b|\beval\s*\(`),
		"runs inline or encoded code"},
}

// scanInstruction returns findings for one instruction file.
func scanInstruction(path, data string) []Finding {
	var out []Finding
	sc := bufio.NewScanner(strings.NewReader(data))
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		hit := map[string]bool{}
		for _, r := range contentRules {
			if hit[r.message] {
				continue
			}
			if r.re.MatchString(line) {
				hit[r.message] = true
				out = append(out, Finding{Severity: r.severity, Category: r.category, Path: path, Line: n,
					Message: r.message, Detail: short(line, 160)})
			}
		}
	}
	return out
}
