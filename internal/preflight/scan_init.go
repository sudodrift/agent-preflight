package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Oversized init scripts used in the October 2026 testmetest campaign were
// 12.9 to 13.7 MB and invisible to GitHub code search. Flag anything at or
// above 1 MiB under .github/init.js, plus smaller files that look like the
// same wrapper.
// Source: https://research.jfrog.com/post/beyond-testmetest-42-hijacked-github-repos/
const initSizeFlagBytes int64 = 1 << 20 // 1 MiB

func scanInitScript(path, data string) []Finding {
	var out []Finding
	st, err := os.Stat(path)
	size := int64(len(data))
	if err == nil {
		size = st.Size()
	}
	if size >= initSizeFlagBytes {
		out = append(out, Finding{Severity: High, Category: "auto-run", Path: path,
			Message: "large script under .github that startup hooks often run",
			Detail:  short(fmt.Sprintf("%s is %s; the Oct 2026 armed-repo campaign used 12.9 to 13.7 MB .github/init.js wrappers", path, humanBytes(size)), 240)})
	}
	lower := strings.ToLower(data)
	if strings.Contains(lower, "eval(") && (strings.Contains(data, "replace(/[a-zA-Z]/g") || strings.Count(data, ",") > 500) {
		out = append(out, Finding{Severity: High, Category: "auto-run", Path: path,
			Message: "obfuscated JavaScript that decodes and evals a payload",
			Detail:  "matches the wrapper skeleton JFrog documented in the testmetest armed-repo campaign"})
	}
	base := filepath.Base(path)
	if (base == "init.js" || base == "init.ps1") && len(out) == 0 && size > 0 {
		out = append(out, Finding{Severity: Medium, Category: "auto-run", Path: path,
			Message: "unexpected " + base + " under .github",
			Detail:  "armed repositories in the Oct 2026 campaign planted .github/init.js for SessionStart and folderOpen hooks"})
	}
	return out
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
