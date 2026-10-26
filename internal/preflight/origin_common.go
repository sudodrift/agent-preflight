//go:build linux || darwin

package preflight

import (
	"strings"
	"unicode"
)

func readAttrs(path string, names []string, get func(string, string, []byte) (int, error)) []Origin {
	var out []Origin
	for _, name := range names {
		buf := make([]byte, 4096)
		n, err := get(path, name, buf)
		if err != nil || n < 0 {
			continue
		}
		if n > len(buf) {
			n = len(buf)
		}
		out = append(out, Origin{Attr: name, Value: printable(string(buf[:n]))})
	}
	return out
}

// printable keeps readable text and replaces the rest, so binary plists and
// control characters do not end up in the terminal.
func printable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('.')
		}
	}
	return short(b.String(), 300)
}
