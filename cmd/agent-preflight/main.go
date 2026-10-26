// Command agent-preflight lists the instruction and config files a coding
// agent would load from a folder, and flags the ones worth a look before you
// start the agent. It only reads files. It never runs anything it finds and
// never sends data anywhere.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sudodrift/agent-preflight/internal/preflight"
)

var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agent-preflight", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print JSON instead of text")
	approve := fs.Bool("approve", false, "record the current files, MCP servers and settings as approved")
	state := fs.String("state", "", "baseline file (default $XDG_STATE_HOME/agent-preflight/baseline.json)")
	home := fs.String("home", "", "home folder to stop the walk at and read global config from (default $HOME)")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: agent-preflight [flags] [folder]\n\n"+
			"Lists what a coding agent would load if started in folder (default: the\n"+
			"current folder) and flags anything worth a look.\n\n"+
			"Exit codes: 0 clean, 1 findings, 2 error.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "agent-preflight", version)
		return 0
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return 2
	}
	start := "."
	if fs.NArg() == 1 {
		start = fs.Arg(0)
	}
	h := *home
	if h == "" {
		var err error
		h, err = os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(stderr, "agent-preflight: cannot find your home folder:", err)
			return 2
		}
	}
	r, base, err := preflight.Scan(preflight.Options{Start: start, Home: h, BaselinePath: *state, Env: os.Getenv, Version: version})
	if err != nil {
		fmt.Fprintln(stderr, "agent-preflight:", err)
		return 2
	}
	if *approve {
		changes := base.Approve(r)
		if err := base.Save(r.Baseline, timeNow()); err != nil {
			fmt.Fprintln(stderr, "agent-preflight: cannot save baseline:", err)
			return 2
		}
		fmt.Fprintf(stderr, "approved %d new or changed items in %s\n", len(changes), r.Baseline)
		for _, c := range changes {
			fmt.Fprintf(stderr, "  %s\n", c)
		}
		return 0
	}
	if *asJSON {
		if err := preflight.WriteJSON(stdout, r); err != nil {
			fmt.Fprintln(stderr, "agent-preflight:", err)
			return 2
		}
	} else {
		preflight.WriteText(stdout, r)
	}
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}
