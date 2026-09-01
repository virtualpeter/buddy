// Package platformschema embeds a snapshot of the Jamf Platform API OpenAPI
// specs so `jamf_schema(api="platform")` works offline.
//
// Unlike Jamf Pro (/api/schema) and Classic (/classicapi/doc/swagger.yaml),
// the Platform API Gateway does not serve a single live schema document. Each
// platform service publishes its own OpenAPI 3.x spec. Official docs and the
// Jamf MCP (developer.jamf.com) can query those APIs but do not host
// downloadable OAS; we vendor the JSON from Jamf-Concepts/jamf-cli (MIT)
// under specs/ and refresh with `make sync-platform-schema`
// (scripts/sync-platform-schema.sh).
//
// TODO: After Platform API Gateway is GA, re-check for a live schema URL
// and consider dropping jamf-cli vendoring. Findings: AGENTS.md
// ("Platform schema vendoring").
package platformschema

import (
	"embed"
	"io/fs"
	"sort"
)

//go:embed all:specs
var specs embed.FS

// Spec is one embedded Platform API OpenAPI document.
type Spec struct {
	// File is the spec filename (e.g. "blueprints_api.json").
	File string
	// Data is the raw JSON document.
	Data []byte
}

// All returns every embedded Platform API spec, sorted by filename.
func All() ([]Spec, error) {
	entries, err := fs.ReadDir(specs, "specs")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	out := make([]Spec, 0, len(names))
	for _, name := range names {
		data, err := specs.ReadFile("specs/" + name)
		if err != nil {
			return nil, err
		}
		out = append(out, Spec{File: name, Data: data})
	}
	return out, nil
}
