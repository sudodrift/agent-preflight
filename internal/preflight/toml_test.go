package preflight

import (
	"reflect"
	"testing"
)

func TestParseTOML(t *testing.T) {
	src := `# comment
model = "o3" # trailing
openai_base_url = 'https://x.example.invalid/v1'

[mcp_servers."my.tools"]
command = "npx"
args = [
  "-y",   # first
  "pkg@1.0",
]
url = "https://h.example.invalid/#frag"

[model_providers.relay]
base_url = "https://relay.example.invalid"

[[hooks.SessionStart]]
command = "./a.sh"
`
	got := map[string]tomlEntry{}
	for _, e := range parseTOML(src) {
		k := e.Key
		for i := len(e.Table) - 1; i >= 0; i-- {
			k = e.Table[i] + "." + k
		}
		got[k] = e
	}
	if got["model"].Value.Str != "o3" {
		t.Errorf("model = %+v", got["model"].Value)
	}
	if got["openai_base_url"].Value.Str != "https://x.example.invalid/v1" || got["openai_base_url"].Line != 3 {
		t.Errorf("openai_base_url = %+v", got["openai_base_url"])
	}
	if e := got["mcp_servers.my.tools.command"]; e.Value.Str != "npx" || len(e.Table) != 2 || e.Table[1] != "my.tools" {
		t.Errorf("quoted table key: %+v", e)
	}
	if e := got["mcp_servers.my.tools.args"]; !reflect.DeepEqual(e.Value.List, []string{"-y", "pkg@1.0"}) {
		t.Errorf("args = %+v", e.Value.List)
	}
	if e := got["mcp_servers.my.tools.url"]; e.Value.Str != "https://h.example.invalid/#frag" {
		t.Errorf("hash inside string: %+v", e.Value)
	}
	if got["model_providers.relay.base_url"].Value.Str != "https://relay.example.invalid" {
		t.Error("model provider base_url missing")
	}
	if got["hooks.SessionStart.command"].Value.Str != "./a.sh" {
		t.Error("array of tables missing")
	}
}
