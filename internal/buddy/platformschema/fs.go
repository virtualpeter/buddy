// Package platformschema embeds a snapshot of the Jamf Platform API OpenAPI
// specs so `jamf_schema(api="platform")` works offline.
//
// Unlike Jamf Pro (/api/schema) and Classic (/classicapi/doc/swagger.yaml),
// the Platform API Gateway does not serve a single live schema document
// (authenticated GETs of /schema and /openapi.json are 404; per-service
// guesses are 403 BAD_PERMISSIONS). Official docs and the Jamf MCP
// (developer.jamf.com) can query those APIs but do not host downloadable
// OAS; we vendor the JSON from Jamf-Concepts/jamfplatform-go-sdk api/
// (MIT) under specs/ and refresh with `make sync-platform-schema`
// (scripts/sync-platform-schema.sh).
//
// Re-check for a live schema URL if Jamf publishes {gateway}/schema or a
// portal openapi.json. Findings: AGENTS.md ("Platform schema vendoring").
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
