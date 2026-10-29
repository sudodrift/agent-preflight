package preflight

import (
	_ "embed"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

//go:embed known-vulnerable.json
var embeddedVulnList []byte

// VulnEntry is one known vulnerable MCP server.
type VulnEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Match    []string `json:"match"`
	Affected string   `json:"affected"`
	Fixed    string   `json:"fixed"`
	Kind     string   `json:"kind"`
	Ref      string   `json:"ref"`
}

// VulnList is the dated list shipped with the binary.
type VulnList struct {
	Updated string      `json:"updated"`
	Entries []VulnEntry `json:"entries"`
}

// LoadVulnList reads the list from path, or the embedded copy if path is "".
func LoadVulnList(path string) (*VulnList, error) {
	data := embeddedVulnList
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = b
	}
	var l VulnList
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// tokenRe splits a command line into package-ish words so "hagent" does
// not match "shagent" but does match "uvx hagent@1.2" or ".../hagent/server.py".
var tokenRe = regexp.MustCompile(`[A-Za-z0-9_\-]+`)

func serverTokens(s MCPServer) []string {
	all := strings.ToLower(s.Name + " " + s.Command + " " + strings.Join(s.Args, " ") + " " + s.URL)
	return tokenRe.FindAllString(all, -1)
}

func tokenMatches(tok, m string) bool {
	if tok == m {
		return true
	}
	// allow version or extension suffixes: mcp-ffmpeg@1.0.5, sf-mcp.js, droidmind-0.4.0
	for _, sep := range []string{"@", ".", "-v", "-0", "-1", "-2", "-3", "-4", "-5", "-6", "-7", "-8", "-9", "==", "_"} {
		if strings.HasPrefix(tok, m+sep) {
			return true
		}
	}
	return false
}

// matchVuln returns the entries that look like they match a server.
func matchVuln(s MCPServer, l *VulnList) []VulnEntry {
	if l == nil {
		return nil
	}
	toks := serverTokens(s)
	// npm scoped names like @ai-sdlc/plugin-mcp-server become "ai-sdlc" tokens already.
	var out []VulnEntry
	for _, e := range l.Entries {
	next:
		for _, m := range e.Match {
			m = strings.ToLower(m)
			for _, t := range toks {
				if tokenMatches(t, m) {
					out = append(out, e)
					break next
				}
			}
		}
	}
	return out
}

func vulnFindings(servers []MCPServer, l *VulnList) []Finding {
	var fs []Finding
	for _, s := range servers {
		for _, e := range matchVuln(s, l) {
			fix := "no fixed release listed; remove it, or run it sandboxed with no credentials"
			if e.Fixed != "" {
				fix = "fixed in " + e.Fixed
			}
			fs = append(fs, Finding{Severity: High, Category: "known-vulnerable", Path: s.Source, Line: s.Line,
				Message: "MCP server \"" + s.Name + "\" looks like " + e.Name + " (" + e.ID + ", affected " + e.Affected + ")",
				Detail:  e.Kind + "; " + fix + "; check the version you run; " + e.Ref + " [list dated " + l.Updated + "]"})
		}
	}
	return fs
}
