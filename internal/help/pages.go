package help

// CLIPage is a help page served by buddy serve / MCP resources.
type CLIPage struct {
	ID    string // URL slug, e.g. "builtins"
	Title string
	Body  string // plain text
}

// CLIPages returns CLI help pages derived from the same strings as `buddy help`.
func CLIPages() []CLIPage {
	return []CLIPage{
		{ID: "root", Title: "CLI overview", Body: Root},
		{ID: "shell", Title: "Shell & scripts", Body: Shell},
		{ID: "starlark", Title: "Starlark language", Body: Starlark},
		{ID: "builtins", Title: "Starlark builtins", Body: Builtins},
	}
}
