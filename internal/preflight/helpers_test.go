package preflight

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyTree copies testdata/<name> into a temp folder, removing the .fixture
// suffix and renaming _git folders to .git. It returns the temp root.
func copyTree(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		rel = strings.ReplaceAll(rel, "_git", ".git")
		rel = strings.TrimSuffix(rel, ".fixture")
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func noEnv(string) string { return "" }

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// scan runs Scan with temp folder detection turned off, because every
// fixture tree lives under the system temp folder.
func scan(t *testing.T, start, home string, env func(string) string) *Report {
	t.Helper()
	r, _, err := scanWithBase(t, start, home, env, filepath.Join(home, ".state", "baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func scanWithBase(t *testing.T, start, home string, env func(string) string, state string) (*Report, *Baseline, error) {
	t.Helper()
	old := tempRoots
	tempRoots = func() []string { return nil }
	defer func() { tempRoots = old }()
	return Scan(Options{Start: start, Home: home, BaselinePath: state, Env: env, Version: "test"})
}

func findings(r *Report, category string) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if category == "" || f.Category == category {
			out = append(out, f)
		}
	}
	return out
}

func hasFinding(r *Report, category, pathSuffix, msgPart string) bool {
	for _, f := range r.Findings {
		if f.Category == category && strings.HasSuffix(f.Path, pathSuffix) && strings.Contains(f.Message, msgPart) {
			return true
		}
	}
	return false
}

func fileByPath(r *Report, suffix string) *File {
	for i := range r.Files {
		if strings.HasSuffix(r.Files[i].Path, suffix) {
			return &r.Files[i]
		}
	}
	return nil
}

func loadStatus(f *File, tool string) string {
	for _, l := range f.LoadedBy {
		if l.Tool == tool {
			return l.Status
		}
	}
	return ""
}

func dump(t *testing.T, r *Report) {
	t.Helper()
	for _, f := range r.Findings {
		t.Logf("%s %s %s:%d %s | %s", f.Severity, f.Category, f.Path, f.Line, f.Message, f.Detail)
	}
}
