package buddy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"buddy/internal/buddy/platformschema"

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
	// Pro / Classic / Platform are set when API is a combined selector ("both" or
	// "all"): per-surface metadata; Document only if full.
	Pro      *SchemaResult `json:"pro,omitempty"`
	Classic  *SchemaResult `json:"classic,omitempty"`
	Platform *SchemaResult `json:"platform,omitempty"`
}

// ResolveSchemaAPI maps user-facing api names to the fetch path and normalized name.
// Combined selectors ("both", "all") return an empty path — use SchemaAPIList
// to expand them. "platform" is served from embedded specs (no path/network).
func ResolveSchemaAPI(api string) (name, path string, err error) {
	switch strings.ToLower(strings.TrimSpace(api)) {
	case "", "pro", "openapi", "jamf", "jamfpro", "modern":
		return "pro", JamfSchemaProPath, nil
	case "classic", "capi", "swagger", "jss":
		return "classic", JamfSchemaClassicPath, nil
	case "platform", "gateway", "apigw", "gw":
		return "platform", "", nil
	case "both":
		return "both", "", nil
	case "all", "everything":
		return "all", "", nil
	default:
		return "", "", fmt.Errorf("api must be \"pro\", \"classic\", \"platform\", \"both\", or \"all\" (got %q)", api)
	}
}

// SchemaAPIList returns the concrete api names to fetch for a user api selector.
// "both" → pro+classic; "all" → pro+classic+platform.
func SchemaAPIList(api string) ([]string, error) {
	name, _, err := ResolveSchemaAPI(api)
	if err != nil {
		return nil, err
	}
	switch name {
	case "both":
		return []string{"pro", "classic"}, nil
	case "all":
		return []string{"pro", "classic", "platform"}, nil
	default:
		return []string{name}, nil
	}
}

// FetchJamfSchema downloads and parses schema(s) for api ("pro", "classic", or "both").
// filter is a case-insensitive expression: space-separated terms are AND-matched
// against path, method, tags, summary, operationId, and description.
// When full is true, Document (or pro/classic/platform.Document) holds the
// filtered schema object. Prefer api="all" (or "both") when discovering whether
// an endpoint lives on Classic, Jamf Pro, or the Platform API Gateway.
func FetchJamfSchema(credName, api, filter string, full bool) (*SchemaResult, error) {
	apis, err := SchemaAPIList(api)
	if err != nil {
		return nil, err
	}
	if len(apis) == 1 {
		return fetchOneJamfSchema(credName, apis[0], filter, full)
	}
	combinedName, _, _ := ResolveSchemaAPI(api)

	var pro, classic, platform *SchemaResult
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
		case "platform":
			platform = res
		}
		ops = append(ops, res.Operations...)
	}
	if pro == nil && classic == nil && platform == nil {
		return nil, fmt.Errorf("fetch schemas: %s", strings.Join(errs, "; "))
	}
	out := &SchemaResult{
		API:        combinedName,
		Count:      len(ops),
		Operations: ops,
		Pro:        stripOps(pro),
		Classic:    stripOps(classic),
		Platform:   stripOps(platform),
	}
	if len(errs) > 0 {
		// Partial success: still return matches; callers see which side failed via nil pro/classic/platform.
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
	switch name {
	case "both", "all":
		return nil, fmt.Errorf("internal: fetchOneJamfSchema does not accept %q", name)
	case "platform":
		// Platform specs are embedded (no live gateway schema endpoint); no cred needed.
		return fetchPlatformSchema(filter, full)
	}

	resp, err := JamfRequest(credName, JamfRequestOpts{
		Method: http.MethodGet,
		Path:   path,
		Accept: "application/json, application/yaml, text/yaml, text/plain, */*",
		// Pro /api/schema and Classic swagger docs are public; no token needed.
		Unauthenticated: true,
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

// fetchPlatformSchema merges the embedded Platform API specs into one catalog.
// The Platform API Gateway has no single live schema document (docs/MCP query
// only), so each service's OpenAPI spec is vendored from jamf-cli (see
// internal/buddy/platformschema). Path keys are prefixed with each spec's
// server base (e.g. /api/blueprints) so they match the real gateway paths, and
// the service segment is added as a tag for filtering.
func fetchPlatformSchema(filter string, full bool) (*SchemaResult, error) {
	specs, err := platformschema.All()
	if err != nil {
		return nil, fmt.Errorf("load platform specs: %w", err)
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("no embedded platform specs found (run 'make sync-platform-schema')")
	}
	var ops []SchemaOperation
	services := map[string]interface{}{}
	for _, sp := range specs {
		doc, err := parseSchemaDocument(sp.Data)
		if err != nil {
			return nil, fmt.Errorf("parse platform spec %s: %w", sp.File, err)
		}
		base := schemaServerBasePath(doc)
		service := strings.TrimPrefix(base, "/api/")
		prefixed := prefixDocPaths(doc, base)
		sops := listSchemaOperations(prefixed, filter)
		for i := range sops {
			sops[i].API = "platform"
			if service != "" && !containsString(sops[i].Tags, service) {
				sops[i].Tags = append(sops[i].Tags, service)
			}
		}
		ops = append(ops, sops...)
		if full && len(sops) > 0 {
			key := service
			if key == "" {
				key = sp.File
			}
			if strings.TrimSpace(filter) == "" {
				services[key] = prefixed
			} else {
				services[key] = filterSchemaDocument(prefixed, filter)
			}
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
	out := &SchemaResult{
		API:        "platform",
		Source:     "embedded: Jamf-Concepts/jamf-cli specs/platform",
		Title:      "Jamf Platform API",
		Spec:       "openapi 3.x",
		Count:      len(ops),
		Operations: ops,
	}
	if full {
		out.Document = map[string]interface{}{"services": services}
	}
	return out, nil
}

// schemaServerBasePath returns the path portion of the first server URL, e.g.
// "https://{region}.apigw.jamf.com/api/blueprints" → "/api/blueprints".
func schemaServerBasePath(doc map[string]interface{}) string {
	servers, _ := doc["servers"].([]interface{})
	if len(servers) == 0 {
		return ""
	}
	s0, _ := servers[0].(map[string]interface{})
	u, _ := s0["url"].(string)
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			return strings.TrimRight(rest[j:], "/")
		}
		return ""
	}
	if strings.HasPrefix(u, "/") {
		return strings.TrimRight(u, "/")
	}
	return ""
}

// prefixDocPaths returns a shallow copy of doc with every path key prefixed by base.
func prefixDocPaths(doc map[string]interface{}, base string) map[string]interface{} {
	paths, _ := doc["paths"].(map[string]interface{})
	if base == "" || paths == nil {
		return doc
	}
	np := make(map[string]interface{}, len(paths))
	for p, v := range paths {
		np[base+p] = v
	}
	out := make(map[string]interface{}, len(doc))
	for k, v := range doc {
		out[k] = v
	}
	out["paths"] = np
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
