package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/docpages"
	"buddy/internal/script"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolResult is the structuredContent envelope for every buddy MCP tool.
// Cursor (and other hosts) expect a JSON object, not a bare array — always wrap lists in Value.
type ToolResult struct {
	OK    bool   `json:"ok"`
	Value any    `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
	Count *int   `json:"count,omitempty"` // set for list-valued tools
}

// Run starts the MCP stdio server until the client disconnects.
func Run(credName string, opts buddy.Opts, logger *slog.Logger, version string) error {
	if logger == nil {
		logger = slog.Default()
	}
	if version == "" {
		version = "dev"
	}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "buddy", Version: version}, nil)
	registerTools(server, credName, opts, logger)
	registerDocResources(server)
	return server.Run(context.Background(), &mcpsdk.StdioTransport{})
}

func registerTools(server *mcpsdk.Server, credName string, opts buddy.Opts, logger *slog.Logger) {
	type credIn struct {
		Cred string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	resolve := func(in credIn) string {
		if in.Cred != "" {
			return in.Cred
		}
		return credName
	}

	type authIn struct {
		Cred    string `json:"cred,omitempty" jsonschema:"credential name override"`
		Verbose bool   `json:"verbose,omitempty" jsonschema:"if true, include full privilege list (large)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "auth_test",
		Description: "Test Jamf authentication (GET /api/v1/auth). Default value is a compact summary; set verbose=true for the full account/privileges payload.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in authIn) (*mcpsdk.CallToolResult, any, error) {
		info, err := buddy.AuthTestValue(resolve(credIn{Cred: in.Cred}), logger)
		if err != nil {
			return toolErr(err)
		}
		if in.Verbose {
			return toolOK(info, nil)
		}
		return toolOK(buddy.CompactAuthInfo(info), nil)
	})

	type filterIn struct {
		Filter   string `json:"filter,omitempty" jsonschema:"name substring or RSQL filter"`
		Page     *int   `json:"page,omitempty" jsonschema:"0-based page; omit to fetch all pages"`
		PageSize int    `json:"page_size,omitempty" jsonschema:"page size (default 200)"`
		Cred     string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	pageOpts := func(in filterIn) buddy.PageOpts {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		return o
	}
	credFromFilter := func(in filterIn) string {
		return resolve(credIn{Cred: in.Cred})
	}

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "smart_groups",
		Description: "List Jamf smart computer groups. Returns {ok, value: [{id,name,description},...], count}. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in filterIn) (*mcpsdk.CallToolResult, any, error) {
		groups, err := buddy.GroupSearchSmart(credFromFilter(in), in.Filter, pageOpts(in), logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(groups)
		return toolOK(groups, &n)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "static_groups",
		Description: "List Jamf static computer groups. Returns {ok, value: [{id,name,description},...], count}. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in filterIn) (*mcpsdk.CallToolResult, any, error) {
		groups, err := buddy.GroupSearchStatic(credFromFilter(in), in.Filter, pageOpts(in), logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(groups)
		return toolOK(groups, &n)
	})

	type computersIn struct {
		Filter   string `json:"filter,omitempty" jsonschema:"name substring or RSQL filter"`
		IDs      bool   `json:"ids,omitempty" jsonschema:"if true, return list of int IDs instead of summary dicts"`
		Page     *int   `json:"page,omitempty" jsonschema:"0-based page; omit to fetch all pages"`
		PageSize int    `json:"page_size,omitempty" jsonschema:"page size (default 200)"`
		Cred     string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "computers",
		Description: "List computers via inventory search. Returns {ok, value: [{id,name,serial,...},...], count}, or IDs if ids=true. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in computersIn) (*mcpsdk.CallToolResult, any, error) {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		recs, err := buddy.ComputerSearch(resolve(credIn{Cred: in.Cred}), in.Filter, o, logger)
		if err != nil {
			return toolErr(err)
		}
		if in.IDs {
			ids := make([]int, 0, len(recs))
			for _, r := range recs {
				ids = append(ids, r.ID)
			}
			n := len(ids)
			return toolOK(ids, &n)
		}
		n := len(recs)
		return toolOK(recs, &n)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "scripts",
		Description: "List Jamf Pro scripts. Returns {ok, value: [...], count}. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in filterIn) (*mcpsdk.CallToolResult, any, error) {
		scripts, err := buddy.ScriptSearch(credFromFilter(in), in.Filter, pageOpts(in), logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(scripts)
		return toolOK(scripts, &n)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "packages",
		Description: "List Jamf packages. Returns {ok, value: [...], count}. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in filterIn) (*mcpsdk.CallToolResult, any, error) {
		pkgs, err := buddy.PackageSearch(credFromFilter(in), in.Filter, pageOpts(in), logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(pkgs)
		return toolOK(pkgs, &n)
	})

	type policyIn struct {
		Trigger string `json:"trigger" jsonschema:"policy trigger name"`
		Cred    string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "policy",
		Description: "Load a Jamf policy by trigger name. Returns {ok, value: {id, trigger}} (ID resolution; use starlark_eval for full details).",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policyIn) (*mcpsdk.CallToolResult, any, error) {
		id, err := buddy.PolicyFind(resolve(credIn{Cred: in.Cred}), opts, in.Trigger, false, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": id, "trigger": in.Trigger}, nil)
	})

	type evalIn struct {
		Source string `json:"source" jsonschema:"Starlark source to evaluate"`
		Cred   string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "starlark_eval",
		Description: "Evaluate Starlark with buddy builtins in a fresh session. " +
			"Returns {ok, value: {stdout, stderr, result, error}}. " +
			"Prefer a single expression, or define main() and it will be called. Top-level if/for are not allowed outside functions.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in evalIn) (*mcpsdk.CallToolResult, any, error) {
		sess, err := script.NewSession(resolve(credIn{Cred: in.Cred}), opts, logger, false)
		if err != nil {
			return toolErr(err)
		}
		res := sess.EvalOpts("mcp", in.Source, script.EvalOptions{CallMain: true})
		out := map[string]any{
			"stdout": res.Stdout,
			"stderr": res.Stderr,
			"result": res.Result,
			"error":  res.Error,
		}
		if res.Error != "" {
			tr := ToolResult{OK: false, Value: out, Error: res.Error}
			return encodeTool(tr), tr, nil
		}
		return toolOK(out, nil)
	})
}

func toolOK(value any, count *int) (*mcpsdk.CallToolResult, any, error) {
	tr := ToolResult{OK: true, Value: value, Count: count}
	return encodeTool(tr), tr, nil
}

func toolErr(err error) (*mcpsdk.CallToolResult, any, error) {
	tr := ToolResult{OK: false, Error: err.Error()}
	res := encodeTool(tr)
	res.IsError = true
	return res, tr, nil
}

func encodeTool(tr ToolResult) *mcpsdk.CallToolResult {
	b, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return &mcpsdk.CallToolResult{
			IsError: true,
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
		}
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(b)}},
	}
}

func registerDocResources(server *mcpsdk.Server) {
	// Full agent knowledge (AGENTS, rules, migration notes, …), embedded by default.
	// BUDDY_DOCS_ROOT still overrides when set. The HTML docs UI uses CatalogHuman instead.
	res, err := docpages.Load(docpages.LoadOptions{Catalog: docpages.CatalogAgent})
	if err != nil {
		return
	}
	for _, p := range res.Pages {
		page := p
		uri := docpages.URI(page)
		mime := "text/markdown"
		if page.Plain {
			mime = "text/plain"
		}
		desc := page.Title
		if strings.HasPrefix(page.ID, "rules/") {
			desc = "Cursor agent rule: " + page.Title
		}
		server.AddResource(&mcpsdk.Resource{
			URI:         uri,
			Name:        page.ID,
			Description: desc,
			MIMEType:    mime,
		}, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
			return &mcpsdk.ReadResourceResult{
				Contents: []*mcpsdk.ResourceContents{{
					URI:      uri,
					MIMEType: mime,
					Text:     page.Markdown,
				}},
			}, nil
		})
		// Backward-compatible aliases for former disk filenames.
		if alias := legacyDocURI(page.ID); alias != "" && alias != uri {
			a := alias
			server.AddResource(&mcpsdk.Resource{
				URI:         a,
				Name:        page.ID + " (alias)",
				Description: desc,
				MIMEType:    mime,
			}, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
				return &mcpsdk.ReadResourceResult{
					Contents: []*mcpsdk.ResourceContents{{
						URI:      a,
						MIMEType: mime,
						Text:     page.Markdown,
					}},
				}, nil
			})
		}
	}
}

func legacyDocURI(id string) string {
	switch id {
	case "docs/index":
		return "buddy://docs/index.md"
	case "docs/reference-manual":
		return "buddy://docs/reference-manual.md"
	case "docs/api-endpoints":
		return "buddy://docs/api-endpoints.md"
	case "docs/mcp":
		return "buddy://docs/mcp.md"
	default:
		return ""
	}
}
