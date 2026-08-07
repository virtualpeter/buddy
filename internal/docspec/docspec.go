// Package docspec holds structured documentation topics colocated with features.
// RenderHuman / RenderAgent produce audience-specific Markdown; gendocs writes them to disk.
package docspec

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// Param describes one CLI flag or builtin parameter for docs and JSON Schema.
type Param struct {
	Name        string // e.g. "filter", "name", "tenant"
	Type        string // e.g. "string", "integer", "boolean"
	Required    bool
	Description string
	Default     string // optional default description
}

// Builtin is a buddyscript function surface.
type Builtin struct {
	Name    string
	Summary string
	Returns string
	Params  []Param
	Notes   []string // human-oriented notes
}

// CLICommand is a buddy CLI command or subcommand.
type CLICommand struct {
	Name    string // e.g. "app list"
	Usage   string
	Summary string
	Params  []Param
	Notes   []string
}

// KindRow is one row in a concept/kinds table.
type KindRow struct {
	Kind    string
	Shape   string
	Upgrade string
}

// RelatedLink is a related doc pointer.
type RelatedLink struct {
	Title string
	Ref   string // markdown link target e.g. "policies.md" or "docs/policies"
}

// Topic is one documentation unit (resource or feature area).
type Topic struct {
	ID       string // catalog id e.g. "docs/apps"
	Title    string
	Section  string
	Keywords []string
	Summary  string // short neutral blurb

	Concepts []string // factual bullets for humans
	Kinds    []KindRow
	CLI      []CLICommand
	Builtins []Builtin

	// AgentNotes are short imperatives for MCP (not shown on human page).
	AgentNotes []string
	// ExamplesBuddy / ExamplesShell are fenced example bodies (without fences).
	ExamplesBuddy []string
	ExamplesShell []string
	Related       []RelatedLink

	// CLIHelp, when set, is emitted verbatim into help (exact CLI wording).
	CLIHelp string
	// BuiltinsHelp, when set, is emitted verbatim as the help.Builtins section block.
	BuiltinsHelp string
	// HelpExport, when set, causes gendocs to write internal/help/<HelpExport>_gen.go
	// with <PascalCase>BuiltinsSection and/or <PascalCase>CLIHelp (e.g. "apps", "policies", "groups").
	HelpExport string
}

var (
	regMu  sync.Mutex
	topics = map[string]Topic{}
)

// Register adds or replaces a topic in the process registry.
func Register(t Topic) {
	if t.ID == "" {
		panic("docspec: topic ID is required")
	}
	regMu.Lock()
	defer regMu.Unlock()
	topics[t.ID] = t
}

// All returns registered topics sorted by ID.
func All() []Topic {
	regMu.Lock()
	defer regMu.Unlock()
	out := make([]Topic, 0, len(topics))
	for _, t := range topics {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns a topic by ID.
func Get(id string) (Topic, bool) {
	regMu.Lock()
	defer regMu.Unlock()
	t, ok := topics[id]
	return t, ok
}

// RenderHuman returns neutral Markdown for humans (buddy serve).
func RenderHuman(t Topic) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(t.Title)
	b.WriteString("\n\n")
	if t.Summary != "" {
		b.WriteString(t.Summary)
		b.WriteString("\n\n")
	}
	if len(t.Kinds) > 0 {
		b.WriteString("## Patterns\n\n")
		b.WriteString("| Kind | Shape | Upgrade |\n")
		b.WriteString("|------|--------|---------|\n")
		for _, k := range t.Kinds {
			b.WriteString("| **")
			b.WriteString(k.Kind)
			b.WriteString("** | ")
			b.WriteString(k.Shape)
			b.WriteString(" | ")
			b.WriteString(k.Upgrade)
			b.WriteString(" |\n")
		}
		b.WriteString("\n")
	}
	if len(t.Concepts) > 0 {
		b.WriteString("## Overview\n\n")
		for _, c := range t.Concepts {
			b.WriteString("- ")
			b.WriteString(c)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(t.ExamplesBuddy) > 0 || len(t.ExamplesShell) > 0 {
		if len(t.ExamplesBuddy)+len(t.ExamplesShell) == 1 {
			b.WriteString("## Example\n\n")
		} else {
			b.WriteString("## Examples\n\n")
		}
		for _, ex := range t.ExamplesBuddy {
			b.WriteString("```python\n")
			b.WriteString(ex)
			b.WriteString("\n```\n\n")
		}
		for _, ex := range t.ExamplesShell {
			b.WriteString("```sh\n")
			b.WriteString(ex)
			b.WriteString("\n```\n\n")
		}
	}
	if len(t.Related) > 0 {
		b.WriteString("## Related\n\n")
		for _, r := range t.Related {
			b.WriteString("- [")
			b.WriteString(r.Title)
			b.WriteString("](")
			b.WriteString(r.Ref)
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

// RenderAgent returns MCP-oriented Markdown: frontmatter, H1–H3, imperatives, JSON Schema.
func RenderAgent(t Topic) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: ")
	b.WriteString(yamlQuote(t.ID))
	b.WriteString("\n")
	b.WriteString("title: ")
	b.WriteString(yamlQuote(t.Title))
	b.WriteString("\n")
	b.WriteString("section: ")
	b.WriteString(yamlQuote(t.Section))
	b.WriteString("\n")
	b.WriteString("audience: agent\n")
	if len(t.Keywords) > 0 {
		b.WriteString("keywords:\n")
		for _, k := range t.Keywords {
			b.WriteString("  - ")
			b.WriteString(yamlQuote(k))
			b.WriteString("\n")
		}
	}
	b.WriteString("---\n\n")
	b.WriteString("# ")
	b.WriteString(t.Title)
	b.WriteString("\n\n")
	if t.Summary != "" {
		b.WriteString(t.Summary)
		b.WriteString("\n\n")
	}
	if len(t.AgentNotes) > 0 {
		b.WriteString("## Agent rules\n\n")
		for _, n := range t.AgentNotes {
			b.WriteString("- ")
			b.WriteString(n)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(t.Concepts) > 0 {
		b.WriteString("## Facts\n\n")
		for _, c := range t.Concepts {
			b.WriteString("- ")
			b.WriteString(c)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(t.Kinds) > 0 {
		b.WriteString("## Kinds\n\n")
		for _, k := range t.Kinds {
			b.WriteString("### ")
			b.WriteString(k.Kind)
			b.WriteString("\n\n")
			b.WriteString(k.Shape)
			b.WriteString("\n\nUpgrade: ")
			b.WriteString(k.Upgrade)
			b.WriteString("\n\n")
		}
	}
	if len(t.Builtins) > 0 {
		b.WriteString("## Builtins\n\n")
		for _, fn := range t.Builtins {
			b.WriteString("### ")
			b.WriteString(fn.Name)
			b.WriteString("\n\n")
			if fn.Summary != "" {
				b.WriteString(fn.Summary)
				b.WriteString("\n\n")
			}
			if fn.Returns != "" {
				b.WriteString("Returns: ")
				b.WriteString(fn.Returns)
				b.WriteString("\n\n")
			}
			b.WriteString("#### Parameters (JSON Schema)\n\n")
			b.WriteString("```json\n")
			b.WriteString(mustJSONSchema(fn.Name, fn.Params))
			b.WriteString("\n```\n\n")
			for _, n := range fn.Notes {
				b.WriteString("- ")
				b.WriteString(n)
				b.WriteString("\n")
			}
			if len(fn.Notes) > 0 {
				b.WriteString("\n")
			}
		}
	}
	if len(t.CLI) > 0 {
		b.WriteString("## CLI\n\n")
		for _, c := range t.CLI {
			b.WriteString("### ")
			b.WriteString(c.Name)
			b.WriteString("\n\n")
			if c.Summary != "" {
				b.WriteString(c.Summary)
				b.WriteString("\n\n")
			}
			if c.Usage != "" {
				b.WriteString("```\n")
				b.WriteString(c.Usage)
				b.WriteString("\n```\n\n")
			}
			if len(c.Params) > 0 {
				b.WriteString("#### Parameters (JSON Schema)\n\n")
				b.WriteString("```json\n")
				b.WriteString(mustJSONSchema(c.Name, c.Params))
				b.WriteString("\n```\n\n")
			}
			for _, n := range c.Notes {
				b.WriteString("- ")
				b.WriteString(n)
				b.WriteString("\n")
			}
			if len(c.Notes) > 0 {
				b.WriteString("\n")
			}
		}
	}
	if len(t.ExamplesBuddy) > 0 || len(t.ExamplesShell) > 0 {
		b.WriteString("## Examples\n\n")
		for _, ex := range t.ExamplesBuddy {
			b.WriteString("```python\n")
			b.WriteString(ex)
			b.WriteString("\n```\n\n")
		}
		for _, ex := range t.ExamplesShell {
			b.WriteString("```sh\n")
			b.WriteString(ex)
			b.WriteString("\n```\n\n")
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*?|-<>=!%@`\"'\n") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func mustJSONSchema(title string, params []Param) string {
	props := map[string]interface{}{}
	var required []string
	for _, p := range params {
		prop := map[string]interface{}{
			"type":        jsonSchemaType(p.Type),
			"description": p.Description,
		}
		if p.Default != "" {
			prop["default"] = p.Default
		}
		props[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := map[string]interface{}{
		"$schema":    "https://json-schema.org/draft/2020-12/schema",
		"title":      title,
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	raw, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return `{"type":"object"}`
	}
	return string(raw)
}

func jsonSchemaType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "int", "integer", "number":
		return "integer"
	case "bool", "boolean":
		return "boolean"
	case "object", "dict":
		return "object"
	case "array", "list":
		return "array"
	default:
		return "string"
	}
}

// RenderBuiltinsHelpFragment returns the POLICY APPS section for help.Builtins.
func RenderBuiltinsHelpFragment(t Topic) string {
	if t.BuiltinsHelp != "" {
		return t.BuiltinsHelp
	}
	var b strings.Builder
	b.WriteString("POLICY APPS (Self Service + packages)\n")
	for _, fn := range t.Builtins {
		b.WriteString("  ")
		b.WriteString(fn.Name)
		b.WriteString("(")
		var parts []string
		for _, p := range fn.Params {
			if p.Required {
				parts = append(parts, p.Name)
			} else {
				parts = append(parts, p.Name+"?")
			}
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString(")")
		if fn.Returns != "" {
			if strings.HasPrefix(fn.Returns, "None") || fn.Returns == "None" {
				b.WriteString(" → None")
			} else {
				b.WriteString("\n      → value: ")
				b.WriteString(fn.Returns)
			}
		}
		b.WriteString("\n")
		if len(fn.Params) > 0 {
			b.WriteString("      Params: ")
			var desc []string
			for _, p := range fn.Params {
				d := p.Name
				if !p.Required {
					d += "?"
				}
				d += ": " + p.Type
				if p.Description != "" {
					d += " (" + p.Description + ")"
				}
				desc = append(desc, d)
			}
			b.WriteString(strings.Join(desc, "; "))
			b.WriteString(".\n")
		}
		for i, n := range fn.Notes {
			if i == 0 {
				b.WriteString("      Notes: ")
				b.WriteString(n)
			} else {
				b.WriteString("\n             ")
				b.WriteString(n)
			}
		}
		if len(fn.Notes) > 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RenderCLIHelp returns full CLI help text for the topic's primary command group.
// When Topic.CLIHelp is set, that text is returned as-is (preferred for exact CLI wording).
func RenderCLIHelp(t Topic) string {
	if t.CLIHelp != "" {
		return t.CLIHelp
	}
	var b strings.Builder
	b.WriteString(t.Title)
	b.WriteString("\n\nUsage:\n  buddy [global-flags] <command> <subcommand> [args...]\n\nSubcommands:\n")
	for _, c := range t.CLI {
		b.WriteString("  ")
		b.WriteString(c.Name)
		b.WriteString(" — ")
		b.WriteString(c.Summary)
		b.WriteString("\n")
	}
	return b.String()
}
