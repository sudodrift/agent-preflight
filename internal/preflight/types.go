package preflight

// Severity of a finding.
const (
	High   = "high"
	Medium = "medium"
	Low    = "low"
)

// Load statuses for a file and a tool.
const (
	Loads       = "loads"       // the docs say the tool loads it in this spot
	Conditional = "conditional" // loads only under a condition named in the note
	Unclear     = "unclear"     // the docs do not say; see docs/sources.md
	NotLoaded   = "not loaded"  // the docs say the tool does not look here
)

// Load says whether one tool would read a file.
type Load struct {
	Tool   string `json:"tool"`
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

// Origin is where a file came from, read from extended attributes.
type Origin struct {
	Attr  string `json:"attr"`
	Value string `json:"value"`
}

// File is one instruction or config file an agent may load.
type File struct {
	Path     string   `json:"path"`
	Kind     string   `json:"kind"`
	Scope    string   `json:"scope"` // start, parent, global
	SHA256   string   `json:"sha256"`
	LoadedBy []Load   `json:"loaded_by"`
	Origins  []Origin `json:"origins,omitempty"`
	Approved bool     `json:"approved"`
}

// Finding is one thing worth a look.
type Finding struct {
	Severity string `json:"severity"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
	Detail   string `json:"detail,omitempty"`
}

// MCPServer is one MCP server definition found in a config file.
type MCPServer struct {
	Name    string   `json:"name"`
	Source  string   `json:"source"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
	Line    int      `json:"line,omitempty"`
}

// Setting is a base URL or proxy value that changes where traffic goes.
type Setting struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
	Line   int    `json:"line,omitempty"`
	Note   string `json:"note,omitempty"`
}

// Report is the full result of one run.
type Report struct {
	Tool        string      `json:"tool"`
	Version     string      `json:"version"`
	StartDir    string      `json:"start_dir"`
	ProjectRoot string      `json:"project_root,omitempty"`
	Home        string      `json:"home"`
	Baseline    string      `json:"baseline"`
	Walked      []string    `json:"walked"`
	Files       []File      `json:"files"`
	MCPServers  []MCPServer `json:"mcp_servers"`
	Settings    []Setting   `json:"settings"`
	Findings    []Finding   `json:"findings"`
	Summary     Summary     `json:"summary"`
}

// Summary counts.
type Summary struct {
	Files    int `json:"files"`
	Findings int `json:"findings"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}
