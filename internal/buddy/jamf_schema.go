package buddy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// JamfSchemaProPath is the OpenAPI 3 document for the Jamf Pro API.
	JamfSchemaProPath = "/api/schema"
	// JamfSchemaClassicPath is the Swagger 2 YAML for the Classic API.
	JamfSchemaClassicPath = "/classicapi/doc/swagger.yaml"
)

// SchemaOperation is one path+method entry from an OpenAPI/Swagger document.
type SchemaOperation struct {
	API         string   `json:"api,omitempty"` // "pro" or "classic" (set when combining)
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	OperationID string   `json:"operationId,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// SchemaResult is the catalog (and optional filtered document) returned to callers.
type SchemaResult struct {
	API        string                 `json:"api"` // "pro", "classic", or "both"
	Source     string                 `json:"source,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Version    string                 `json:"version,omitempty"`
	Spec       string                 `json:"spec,omitempty"` // e.g. openapi 3.0.1 or swagger 2.0
	Count      int                    `json:"count"`
	Operations []SchemaOperation      `json:"operations"`
	Document   map[string]interface{} `json:"document,omitempty"`
	// Pro / Classic are set when API == "both" (per-surface metadata; Document only if full).
	Pro     *SchemaResult `json:"pro,omitempty"`
	Classic *SchemaResult `json:"classic,omitempty"`
}

// ResolveSchemaAPI maps user-facing api names to the fetch path and normalized name.
// For "both"/"all", name is "both" and path is empty — use SchemaAPIList instead.
func ResolveSchemaAPI(api string) (name, path string, err error) {
	switch strings.ToLower(strings.TrimSpace(api)) {
	case "", "pro", "openapi", "jamf", "jamfpro", "modern":
		return "pro", JamfSchemaProPath, nil
	case "classic", "capi", "swagger", "jss":
		return "classic", JamfSchemaClassicPath, nil
	case "both", "all":
		return "both", "", nil
	default:
		return "", "", fmt.Errorf("api must be \"pro\", \"classic\", or \"both\" (got %q)", api)
	}
}

// SchemaAPIList returns the concrete api names to fetch for a user api selector.
func SchemaAPIList(api string) ([]string, error) {
	name, _, err := ResolveSchemaAPI(api)
	if err != nil {
		return nil, err
	}
	if name == "both" {
		return []string{"pro", "classic"}, nil
	}
	return []string{name}, nil
}

// FetchJamfSchema downloads and parses schema(s) for api ("pro", "classic", or "both").
// filter is a case-insensitive expression: space-separated terms are AND-matched
// against path, method, tags, summary, operationId, and description.
// When full is true, Document (or pro/classic.Document) holds the filtered schema object.
// Prefer api="both" when discovering whether an endpoint lives on Classic or Jamf Pro API.
func FetchJamfSchema(credName, api, filter string, full bool) (*SchemaResult, error) {
	apis, err := SchemaAPIList(api)
	if err != nil {
		return nil, err
	}
	if len(apis) == 1 {
		return fetchOneJamfSchema(credName, apis[0], filter, full)
	}

	var pro, classic *SchemaResult
	var ops []SchemaOperation
	var errs []string
	for _, a := range apis {
		res, err := fetchOneJamfSchema(credName, a, filter, full)
		if err != nil {
			errs = append(errs, a+": "+err.Error())
			continue
		}
		switch res.API {
		case "pro":
			pro = res
		case "classic":
			classic = res
		}
		ops = append(ops, res.Operations...)
	}
	if pro == nil && classic == nil {
		return nil, fmt.Errorf("fetch schemas: %s", strings.Join(errs, "; "))
	}
	out := &SchemaResult{
		API:        "both",
		Count:      len(ops),
		Operations: ops,
		Pro:        stripOps(pro),
		Classic:    stripOps(classic),
	}
	if len(errs) > 0 {
		// Partial success: still return matches; callers see which side failed via nil pro/classic.
		out.Title = "partial: " + strings.Join(errs, "; ")
	}
	return out, nil
}

// stripOps returns a copy without the operations list (kept on the combined result).
func stripOps(r *SchemaResult) *SchemaResult {
	if r == nil {
		return nil
	}
	cp := *r
	cp.Operations = nil
	return &cp
}

func fetchOneJamfSchema(credName, api, filter string, full bool) (*SchemaResult, error) {
	name, path, err := ResolveSchemaAPI(api)
	if err != nil {
		return nil, err
	}
	if name == "both" {
		return nil, fmt.Errorf("internal: fetchOneJamfSchema does not accept both")
	}

	resp, err := JamfRequest(credName, JamfRequestOpts{
		Method: http.MethodGet,
		Path:   path,
		Accept: "application/json, application/yaml, text/yaml, text/plain, */*",
	})
	if err != nil {
		return nil, err
	}
	if resp.Status < 200 || resp.Status >= 300 {
		preview := strings.TrimSpace(string(resp.Body))
		if len(preview) > 300 {
			preview = preview[:300] + "…"
		}
		return nil, fmt.Errorf("HTTP %d GET %s: %s", resp.Status, path, preview)
	}

	doc, err := parseSchemaDocument(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse schema from %s: %w", path, err)
	}

	ops := listSchemaOperations(doc, filter)
	for i := range ops {
		ops[i].API = name
	}
	out := &SchemaResult{
		API:        name,
		Source:     path,
		Title:      schemaInfoString(doc, "title"),
		Version:    schemaInfoString(doc, "version"),
		Spec:       schemaSpecVersion(doc),
		Count:      len(ops),
		Operations: ops,
	}
	if full {
		if strings.TrimSpace(filter) == "" {
			out.Document = doc
		} else {
			out.Document = filterSchemaDocument(doc, filter)
		}
	}
	return out, nil
}

func parseSchemaDocument(data []byte) (map[string]interface{}, error) {
	trim := strings.TrimLeft(string(data), " \t\r\n")
	var doc map[string]interface{}
	if strings.HasPrefix(trim, "{") {
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
		return doc, nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func schemaInfoString(doc map[string]interface{}, key string) string {
	info, _ := doc["info"].(map[string]interface{})
	if info == nil {
		return ""
	}
	v, _ := info[key].(string)
	return v
}

func schemaSpecVersion(doc map[string]interface{}) string {
	if v, ok := doc["openapi"].(string); ok && v != "" {
		return "openapi " + v
	}
	if v, ok := doc["swagger"].(string); ok && v != "" {
		return "swagger " + v
	}
	return ""
}

func listSchemaOperations(doc map[string]interface{}, filter string) []SchemaOperation {
	paths, _ := doc["paths"].(map[string]interface{})
	if paths == nil {
		return nil
	}
	terms := filterTerms(filter)
	var out []SchemaOperation
	pathKeys := make([]string, 0, len(paths))
	for p := range paths {
		pathKeys = append(pathKeys, p)
	}
	sort.Strings(pathKeys)
	for _, path := range pathKeys {
		item, ok := paths[path].(map[string]interface{})
		if !ok {
			continue
		}
		methodKeys := make([]string, 0, len(item))
		for m := range item {
			methodKeys = append(methodKeys, m)
		}
		sort.Strings(methodKeys)
		for _, method := range methodKeys {
			if !isHTTPMethod(method) {
				continue
			}
			opMap, _ := item[method].(map[string]interface{})
			op := schemaOpFromMap(path, method, opMap)
			if !schemaOpMatches(op, terms) {
				continue
			}
			out = append(out, op)
		}
	}
	return out
}

func filterSchemaDocument(doc map[string]interface{}, filter string) map[string]interface{} {
	terms := filterTerms(filter)
	paths, _ := doc["paths"].(map[string]interface{})
	filteredPaths := map[string]interface{}{}
	for path, item := range paths {
		methods, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		kept := map[string]interface{}{}
		for method, raw := range methods {
			if !isHTTPMethod(method) {
				continue
			}
			opMap, _ := raw.(map[string]interface{})
			op := schemaOpFromMap(path, method, opMap)
			if schemaOpMatches(op, terms) {
				kept[method] = raw
			}
		}
		if len(kept) == 0 {
			continue
		}
		// Preserve path-level parameters when present.
		if p, ok := methods["parameters"]; ok {
			kept["parameters"] = p
		}
		if r, ok := methods["$ref"]; ok {
			kept["$ref"] = r
		}
		filteredPaths[path] = kept
	}
	out := make(map[string]interface{}, len(doc))
	for k, v := range doc {
		out[k] = v
	}
	out["paths"] = filteredPaths
	return out
}

func schemaOpFromMap(path, method string, op map[string]interface{}) SchemaOperation {
	out := SchemaOperation{
		Path:   path,
		Method: strings.ToLower(method),
	}
	if op == nil {
		return out
	}
	if s, ok := op["summary"].(string); ok {
		out.Summary = s
	}
	if s, ok := op["description"].(string); ok {
		out.Description = s
	}
	if s, ok := op["operationId"].(string); ok {
		out.OperationID = s
	}
	if tags, ok := op["tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				out.Tags = append(out.Tags, s)
			}
		}
	}
	return out
}

func filterTerms(filter string) []string {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return nil
	}
	return strings.Fields(filter)
}

func schemaOpMatches(op SchemaOperation, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	var b strings.Builder
	b.WriteString(op.Path)
	b.WriteByte(' ')
	b.WriteString(op.Method)
	b.WriteByte(' ')
	b.WriteString(op.Summary)
	b.WriteByte(' ')
	b.WriteString(op.Description)
	b.WriteByte(' ')
	b.WriteString(op.OperationID)
	for _, t := range op.Tags {
		b.WriteByte(' ')
		b.WriteString(t)
	}
	hay := strings.ToLower(b.String())
	for _, term := range terms {
		if !strings.Contains(hay, term) {
			return false
		}
	}
	return true
}

func isHTTPMethod(m string) bool {
	switch strings.ToLower(m) {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	default:
		return false
	}
}
