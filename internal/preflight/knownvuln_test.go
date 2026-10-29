package preflight

import "testing"

func TestMatchVuln(t *testing.T) {
	l, err := LoadVulnList("")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) < 10 || l.Updated == "" {
		t.Fatalf("embedded list looks wrong: %+v", l)
	}
	cases := []struct {
		s    MCPServer
		want string
	}{
		{MCPServer{Name: "video", Command: "npx", Args: []string{"-y", "mcp-ffmpeg@1.0.5"}}, "CVE-2026-117568"},
		{MCPServer{Name: "sf", Command: "npx", Args: []string{"sf-mcp"}}, "CVE-2026-117561"},
		{MCPServer{Name: "aws", Command: "uvx", Args: []string{"awslabs.aws-api-mcp-server@1.5.0"}}, "CVE-2026-115413"},
		{MCPServer{Name: "phone", Command: "python", Args: []string{"/home/u/src/droidmind/server.py"}}, "CVE-2026-117559"},
		{MCPServer{Name: "x", Command: "npx", Args: []string{"@ai-sdlc/plugin-mcp-server"}}, "CVE-2026-117558"},
	}
	for _, c := range cases {
		got := matchVuln(c.s, l)
		if len(got) != 1 || got[0].ID != c.want {
			t.Errorf("%+v: got %+v, want %s", c.s, got, c.want)
		}
	}
	for _, s := range []MCPServer{
		{Name: "fs", Command: "npx", Args: []string{"@modelcontextprotocol/server-filesystem"}},
		{Name: "shagent", Command: "/opt/shagent/run"},
		{Name: "nmap-mcpx", Command: "nmap-mcpx"},
	} {
		if got := matchVuln(s, l); len(got) != 0 {
			t.Errorf("%+v: unexpected match %+v", s, got)
		}
	}
}
