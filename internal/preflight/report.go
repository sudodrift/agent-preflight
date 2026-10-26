package preflight

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// WriteJSON prints the report as indented JSON.
func WriteJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

func loc(path string, line int) string {
	if line > 0 {
		return fmt.Sprintf("%s:%d", path, line)
	}
	return path
}

// WriteText prints the report for a person to read.
func WriteText(w io.Writer, r *Report) {
	fmt.Fprintf(w, "agent-preflight %s\n", r.Version)
	fmt.Fprintf(w, "start:    %s\n", r.StartDir)
	if r.ProjectRoot != "" {
		fmt.Fprintf(w, "project:  %s\n", r.ProjectRoot)
	} else {
		fmt.Fprintf(w, "project:  none found (no .git above the start folder)\n")
	}
	if n := len(r.Walked); n > 0 {
		fmt.Fprintf(w, "walked:   %d folders, up to %s\n", n, r.Walked[n-1])
	}
	fmt.Fprintf(w, "baseline: %s\n\n", r.Baseline)

	fmt.Fprintf(w, "Files agents would load (%d)\n", len(r.Files))
	if len(r.Files) == 0 {
		fmt.Fprintln(w, "  none")
	}
	for _, f := range r.Files {
		mark := ""
		if f.Approved {
			mark = "  (approved)"
		}
		fmt.Fprintf(w, "  %s  [%s, %s]%s\n", f.Path, f.Kind, f.Scope, mark)
		var parts []string
		for _, l := range f.LoadedBy {
			if l.Status == NotLoaded {
				continue
			}
			parts = append(parts, l.Tool+": "+l.Status)
		}
		if len(parts) > 0 {
			fmt.Fprintf(w, "      %s\n", strings.Join(parts, "; "))
		}
		for _, o := range f.Origins {
			fmt.Fprintf(w, "      from: %s = %s\n", o.Attr, o.Value)
		}
	}

	if len(r.MCPServers) > 0 {
		fmt.Fprintf(w, "\nMCP servers (%d)\n", len(r.MCPServers))
		for _, s := range r.MCPServers {
			fmt.Fprintf(w, "  %s  %s  (%s)\n", s.Name, describeServer(s), loc(s.Source, s.Line))
		}
	}
	if len(r.Settings) > 0 {
		fmt.Fprintf(w, "\nBase URL, proxy and config folder settings (%d)\n", len(r.Settings))
		for _, s := range r.Settings {
			fmt.Fprintf(w, "  %s = %s  (%s)\n", s.Key, s.Value, loc(s.Source, s.Line))
		}
	}

	fmt.Fprintf(w, "\nFindings (%d)\n", len(r.Findings))
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "  none")
	}
	for _, f := range r.Findings {
		fmt.Fprintf(w, "  %-6s %-13s %s\n", strings.ToUpper(f.Severity), f.Category, loc(f.Path, f.Line))
		fmt.Fprintf(w, "         %s\n", f.Message)
		if f.Detail != "" {
			fmt.Fprintf(w, "         > %s\n", f.Detail)
		}
	}
	fmt.Fprintf(w, "\nRESULT: ")
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "clean")
	} else {
		noun := "findings"
		if r.Summary.Findings == 1 {
			noun = "finding"
		}
		fmt.Fprintf(w, "%d %s (%d high, %d medium, %d low). Look at each one before you start the agent.\n",
			r.Summary.Findings, noun, r.Summary.High, r.Summary.Medium, r.Summary.Low)
		fmt.Fprintln(w, "If everything above is yours and expected, run again with --approve to record it.")
	}
}
