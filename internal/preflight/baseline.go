package preflight

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Baseline is the state file of things you have approved.
type Baseline struct {
	Version int    `json:"version"`
	Updated string `json:"updated"`
	// Files maps an absolute path to the SHA-256 of the approved content.
	Files map[string]string `json:"files"`
	// MCPServers maps a server fingerprint to its name.
	MCPServers map[string]string `json:"mcp_servers"`
	// Settings maps "key|source" to the approved value.
	Settings map[string]string `json:"settings"`
}

func newBaseline() *Baseline {
	return &Baseline{Version: 1, Files: map[string]string{}, MCPServers: map[string]string{}, Settings: map[string]string{}}
}

// DefaultBaselinePath is $XDG_STATE_HOME/agent-preflight/baseline.json, or
// ~/.local/state/agent-preflight/baseline.json.
func DefaultBaselinePath(home string, env func(string) string) string {
	if x := env("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "agent-preflight", "baseline.json")
	}
	return filepath.Join(home, ".local", "state", "agent-preflight", "baseline.json")
}

// LoadBaseline reads the state file. A missing file is an empty baseline.
func LoadBaseline(path string) (*Baseline, error) {
	b := newBaseline()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, b); err != nil {
		return nil, errors.New("baseline " + path + " is not valid JSON: " + err.Error())
	}
	if b.Files == nil {
		b.Files = map[string]string{}
	}
	if b.MCPServers == nil {
		b.MCPServers = map[string]string{}
	}
	if b.Settings == nil {
		b.Settings = map[string]string{}
	}
	return b, nil
}

// Save writes the state file with owner-only permissions.
func (b *Baseline) Save(path string, now time.Time) error {
	b.Updated = now.UTC().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fingerprint(s MCPServer) string {
	h := sha256.Sum256([]byte(strings.Join([]string{s.Name, s.Command, strings.Join(s.Args, "\x1f"), s.URL}, "\x00")))
	return hex.EncodeToString(h[:])
}

func settingKey(s Setting) string { return s.Key + "|" + s.Source }

func (b *Baseline) knownServerName(name string) bool {
	for _, n := range b.MCPServers {
		if n == name {
			return true
		}
	}
	return false
}

// Approve records everything in the report as approved.
func (b *Baseline) Approve(r *Report) []string {
	var changes []string
	for _, f := range r.Files {
		if b.Files[f.Path] != f.SHA256 {
			changes = append(changes, "file "+f.Path)
		}
		b.Files[f.Path] = f.SHA256
	}
	for _, s := range r.MCPServers {
		fp := fingerprint(s)
		if _, ok := b.MCPServers[fp]; !ok {
			changes = append(changes, "MCP server "+s.Name+" ("+s.Source+")")
		}
		b.MCPServers[fp] = s.Name
	}
	for _, s := range r.Settings {
		if v, ok := b.Settings[settingKey(s)]; !ok || v != s.Value {
			changes = append(changes, "setting "+s.Key+" in "+s.Source)
		}
		b.Settings[settingKey(s)] = s.Value
	}
	sort.Strings(changes)
	return changes
}
