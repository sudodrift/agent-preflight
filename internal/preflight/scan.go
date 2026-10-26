package preflight

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options for one run.
type Options struct {
	Start        string
	Home         string
	BaselinePath string
	Env          func(string) string
	Version      string
}

const maxRead = 4 << 20 // 4 MiB is the most any tool here loads from one file

func canon(p string) (string, error) {
	a, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(a); err == nil {
		return r, nil
	}
	return a, nil
}

func readCapped(p string) (string, string, error) {
	fh, err := os.Open(p)
	if err != nil {
		return "", "", err
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, maxRead))
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	return string(data), hex.EncodeToString(sum[:]), nil
}

// Scan walks from the start folder up to home, reads what agents would load
// and returns a report. It never runs anything it finds.
func Scan(o Options) (*Report, *Baseline, error) {
	if o.Env == nil {
		o.Env = os.Getenv
	}
	start, err := canon(o.Start)
	if err != nil {
		return nil, nil, err
	}
	st, err := os.Stat(start)
	if err != nil {
		return nil, nil, err
	}
	if !st.IsDir() {
		return nil, nil, errors.New(start + " is not a folder")
	}
	home, err := canon(o.Home)
	if err != nil {
		return nil, nil, err
	}
	bpath := o.BaselinePath
	if bpath == "" {
		bpath = DefaultBaselinePath(home, o.Env)
	}
	base, err := LoadBaseline(bpath)
	if err != nil {
		return nil, nil, err
	}

	ctx := &walkContext{start: start, home: home, projectRoot: findProjectRoot(start)}
	ctx.dirs = ancestors(start, home)
	items := discover(ctx, o.Env)
	for _, it := range items {
		if it.scope != "global" && (it.kind == KindClaudeMD || it.kind == KindClaudeLocalMD) {
			ctx.claudeMDSet = true
		}
	}

	r := &Report{Tool: "agent-preflight", Version: o.Version, StartDir: start, ProjectRoot: ctx.projectRoot,
		Home: home, Baseline: bpath, Walked: ctx.dirs, Files: []File{}, MCPServers: []MCPServer{},
		Settings: []Setting{}, Findings: []Finding{}}

	for _, it := range items {
		data, sum, err := readCapped(it.path)
		if err != nil {
			r.Findings = append(r.Findings, Finding{Severity: Low, Category: "read", Path: it.path,
				Message: "could not read this file", Detail: err.Error()})
			continue
		}
		f := File{Path: it.path, Kind: it.kind, Scope: it.scope, SHA256: sum,
			LoadedBy: loadsFor(it, ctx), Origins: readOrigins(it.path)}
		f.Approved = base.Files[f.Path] == sum
		var fileFindings []Finding

		if isInstruction(it.kind) {
			fileFindings = append(fileFindings, scanInstruction(it.path, data)...)
		}
		var cr configResult
		switch it.kind {
		case KindCodexConfig:
			cr = parseCodexConfig(it.path, data, it.scope != "global")
		case KindMCPJSON, KindVSCodeMCP, KindClaudeSettings, KindClaudeLocal, KindClaudeUserJSON, KindCodexHooks, KindCopilotMCPGlobal:
			cr = parseJSONConfig(it.path, it.kind, data, &f, ctx)
		}
		r.MCPServers = append(r.MCPServers, cr.servers...)
		r.Settings = append(r.Settings, cr.settings...)
		fileFindings = append(fileFindings, cr.findings...)

		for _, o := range f.Origins {
			fileFindings = append(fileFindings, Finding{Severity: High, Category: "origin", Path: it.path,
				Message: "this file was downloaded from the internet (" + o.Attr + ")", Detail: o.Value})
		}
		if place, sev := unusualPlace(it.path, home); place != "" && anyLoad(f.LoadedBy) {
			fileFindings = append(fileFindings, Finding{Severity: sev, Category: "location", Path: it.path,
				Message: "agent file in " + place})
		} else if it.scope == "parent" && anyLoad(f.LoadedBy) {
			if ctx.projectRoot != "" && !isWithin(it.dir, ctx.projectRoot) {
				fileFindings = append(fileFindings, Finding{Severity: Medium, Category: "location", Path: it.path,
					Message: "sits above the project root (" + ctx.projectRoot + ") but some agents still load it"})
			} else if ctx.projectRoot == "" {
				fileFindings = append(fileFindings, Finding{Severity: Low, Category: "location", Path: it.path,
					Message: "sits in a parent folder and no project root (.git) was found"})
			}
		}
		if !f.Approved {
			r.Findings = append(r.Findings, fileFindings...)
		}
		r.Files = append(r.Files, f)
	}

	for _, k := range append(append([]string{}, baseURLEnvKeys...), configDirEnvKeys...) {
		if v := o.Env(k); v != "" {
			s := Setting{Key: k, Value: v, Source: "environment"}
			if k == "CLAUDE_CONFIG_DIR" || k == "CODEX_HOME" {
				s.Note = "moves where the agent reads its user config from"
			}
			r.Settings = append(r.Settings, s)
		}
	}

	for _, s := range r.MCPServers {
		if _, ok := base.MCPServers[fingerprint(s)]; ok {
			continue
		}
		msg := "MCP server \"" + s.Name + "\" is not in your approved baseline"
		if base.knownServerName(s.Name) {
			msg = "MCP server \"" + s.Name + "\" changed since you approved it"
		}
		sev := High
		detail := describeServer(s)
		if risky := riskyLauncher(s); risky != "" {
			detail += " [" + risky + "]"
		}
		r.Findings = append(r.Findings, Finding{Severity: sev, Category: "mcp-server", Path: s.Source, Line: s.Line,
			Message: msg, Detail: detail})
	}
	for _, s := range r.Settings {
		old, ok := base.Settings[settingKey(s)]
		if ok && old == s.Value {
			continue
		}
		msg := s.Key + " is set"
		if ok {
			msg = s.Key + " changed since you approved it (was " + old + ")"
		}
		detail := s.Value
		if s.Note != "" {
			detail += " (" + s.Note + ")"
		}
		r.Findings = append(r.Findings, Finding{Severity: High, Category: "base-url", Path: s.Source, Line: s.Line,
			Message: msg, Detail: detail})
	}

	if place, sev := unusualPlace(start, home); place != "" {
		r.Findings = append(r.Findings, Finding{Severity: sev, Category: "location", Path: start,
			Message: "the start folder is in " + place + "; files there can arrive without you putting them there"})
	} else if start == home {
		r.Findings = append(r.Findings, Finding{Severity: Medium, Category: "location", Path: start,
			Message: "the start folder is your home folder; agents treat everything in it as the project"})
	}

	sortFindings(r.Findings)
	r.Summary.Files = len(r.Files)
	r.Summary.Findings = len(r.Findings)
	for _, f := range r.Findings {
		switch f.Severity {
		case High:
			r.Summary.High++
		case Medium:
			r.Summary.Medium++
		default:
			r.Summary.Low++
		}
	}
	return r, base, nil
}

func anyLoad(ls []Load) bool {
	for _, l := range ls {
		if l.Status != NotLoaded {
			return true
		}
	}
	return false
}

func describeServer(s MCPServer) string {
	if s.URL != "" {
		return "url " + s.URL
	}
	return "command " + short(strings.TrimSpace(s.Command+" "+strings.Join(s.Args, " ")), 200)
}

// riskyLauncher notes server commands that hand off to a shell, a scheduler
// or a package runner, which is how the payload in the Forever write-up
// started its install.
func riskyLauncher(s MCPServer) string {
	cmd := filepath.Base(s.Command)
	all := " " + strings.Join(s.Args, " ") + " "
	switch cmd {
	case "launchctl", "crontab", "schtasks", "systemd-run":
		return "starts a scheduler, not a server"
	case "sh", "bash", "zsh", "dash", "cmd", "cmd.exe", "powershell", "pwsh":
		return "runs a shell command"
	}
	if strings.Contains(all, " -c ") || strings.Contains(all, " -lc ") {
		return "runs a shell command"
	}
	if cmd == "npx" || cmd == "uvx" || cmd == "bunx" || cmd == "pnpx" {
		return "downloads and runs a package"
	}
	if strings.Contains(all, "npm install") || strings.Contains(all, "curl ") || strings.Contains(all, "wget ") {
		return "installs or downloads at startup"
	}
	return ""
}

// tempRoots lists shared temp folders. Tests replace it, because their
// fixture trees live under the system temp folder.
var tempRoots = func() []string {
	tmps := []string{"/tmp", "/var/tmp", "/dev/shm", "/private/tmp", "/private/var/folders"}
	if t := os.TempDir(); t != "" {
		if c, err := canon(t); err == nil {
			tmps = append(tmps, c)
		}
	}
	return tmps
}

// unusualPlace says whether a path is somewhere the web or other users can
// drop files.
func unusualPlace(p, home string) (string, string) {
	lower := strings.ToLower(filepath.ToSlash(p))
	for _, seg := range strings.Split(lower, "/") {
		switch seg {
		case "downloads":
			return "a Downloads folder", High
		case ".trash":
			return "the trash", High
		}
	}
	if strings.Contains(lower, "/.local/share/trash/") {
		return "the trash", High
	}
	for _, t := range tempRoots() {
		if isWithin(p, t) {
			return "a temp folder (" + t + ")", High
		}
	}
	for _, seg := range strings.Split(lower, "/") {
		if seg == "desktop" {
			return "the Desktop folder", Medium
		}
	}
	return "", ""
}

func sevRank(s string) int {
	switch s {
	case High:
		return 0
	case Medium:
		return 1
	}
	return 2
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if sevRank(a.Severity) != sevRank(b.Severity) {
			return sevRank(a.Severity) < sevRank(b.Severity)
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
}
