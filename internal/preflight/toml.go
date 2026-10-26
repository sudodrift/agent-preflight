package preflight

import (
	"bufio"
	"strings"
)

// tomlEntry is one key = value pair with the table it sits in.
type tomlEntry struct {
	Table []string // table path, for example ["mcp_servers", "docs"]
	Key   string
	Value tomlValue
	Line  int
}

// tomlValue is a string, a list of strings, or raw text for anything else.
type tomlValue struct {
	Str    string
	List   []string
	IsStr  bool
	IsList bool
	Raw    string
}

// parseTOML reads the small subset of TOML that agent configs use: tables,
// arrays of tables, dotted keys, strings, arrays of strings (also across
// lines) and plain values. It never fails; lines it does not understand are
// skipped. It is enough to find MCP servers, base URLs, hooks and notify.
func parseTOML(data string) []tomlEntry {
	var out []tomlEntry
	var table []string
	sc := bufio.NewScanner(strings.NewReader(data))
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	lineNo := 0
	pending := ""
	pendingLine := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if pending != "" {
			pending += "\n" + stripComment(line)
			if !bracketsOpen(pending) {
				out = append(out, kvEntry(table, pending, pendingLine)...)
				pending = ""
			}
			continue
		}
		t := strings.TrimSpace(stripComment(line))
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "[[") && strings.HasSuffix(t, "]]") {
			table = splitKey(strings.TrimSpace(t[2 : len(t)-2]))
			continue
		}
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			table = splitKey(strings.TrimSpace(t[1 : len(t)-1]))
			continue
		}
		if !strings.Contains(t, "=") {
			continue
		}
		if bracketsOpen(t) {
			pending = t
			pendingLine = lineNo
			continue
		}
		out = append(out, kvEntry(table, t, lineNo)...)
	}
	return out
}

func kvEntry(table []string, t string, line int) []tomlEntry {
	i := indexOutsideQuotes(t, '=')
	if i < 0 {
		return nil
	}
	keyPath := splitKey(strings.TrimSpace(t[:i]))
	if len(keyPath) == 0 {
		return nil
	}
	val := parseTOMLValue(strings.TrimSpace(stripComment(t[i+1:])))
	full := append(append([]string{}, table...), keyPath[:len(keyPath)-1]...)
	return []tomlEntry{{Table: full, Key: keyPath[len(keyPath)-1], Value: val, Line: line}}
}

func parseTOMLValue(v string) tomlValue {
	tv := tomlValue{Raw: v}
	if s, ok := unquoteTOML(v); ok {
		tv.Str, tv.IsStr = s, true
		return tv
	}
	if strings.HasPrefix(v, "[") {
		tv.IsList = true
		for _, part := range splitTopLevel(strings.TrimSuffix(strings.TrimPrefix(v, "["), "]")) {
			p := strings.TrimSpace(stripComment(part))
			if p == "" {
				continue
			}
			if s, ok := unquoteTOML(p); ok {
				tv.List = append(tv.List, s)
			} else {
				tv.List = append(tv.List, p)
			}
		}
	}
	return tv
}

func unquoteTOML(v string) (string, bool) {
	if len(v) >= 6 && (strings.HasPrefix(v, `"""`) && strings.HasSuffix(v, `"""`) || strings.HasPrefix(v, `'''`) && strings.HasSuffix(v, `'''`)) {
		return v[3 : len(v)-3], true
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return v[1 : len(v)-1], true
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		r := strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\t`, "\t")
		return r.Replace(v[1 : len(v)-1]), true
	}
	return "", false
}

// splitKey splits a dotted key, keeping quoted parts whole.
func splitKey(k string) []string {
	var parts []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '.':
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" || len(parts) > 0 {
		parts = append(parts, s)
	}
	return parts
}

func stripComment(s string) string {
	if i := indexOutsideQuotes(s, '#'); i >= 0 {
		return s[:i]
	}
	return s
}

func indexOutsideQuotes(s string, target byte) int {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == target {
			return i
		}
	}
	return -1
}

func bracketsOpen(s string) bool {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '#':
			// skip to end of line
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
	}
	return depth > 0
}

func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	var quote byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}
