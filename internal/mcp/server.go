package mcp

import (
	"context"
	"encoding/json"
	"fmt"
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

// withMCPOpts forces SkipConfirm. MCP has no interactive stdin / AnswerConfirm
// path (unlike notebook/IDE); without this, Classic mutates such as
// profile_put_xml block forever on the session confirm bridge and hosts time out.
func withMCPOpts(opts buddy.Opts) buddy.Opts {
	opts.SkipConfirm = true
	return opts
}

// Run starts the MCP stdio server until the client disconnects.
func Run(credName string, opts buddy.Opts, logger *slog.Logger, version string) error {
	if logger == nil {
		logger = slog.Default()
	}
	if version == "" {
		version = "dev"
	}
	opts = withMCPOpts(opts)
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "buddy", Version: version}, &mcpsdk.ServerOptions{
		Instructions: ServerInstructions,
	})
	registerTools(server, credName, opts, logger, version)
	registerDocResources(server)
	return server.Run(context.Background(), &mcpsdk.StdioTransport{})
}

func registerTools(server *mcpsdk.Server, credName string, opts buddy.Opts, logger *slog.Logger, version string) {
	resolve := func(in sessionIn) string {
		if in.Auth != "" {
			return in.Auth
		}
		if in.Tenant != "" {
			return ""
		}
		return credName
	}
	sessFrom := func(tenant, surface, auth string) sessionIn {
		return sessionIn{Tenant: tenant, Surface: surface, Auth: auth}
	}
	// applySession sets overrides for the duration of the current tool call.
	// Use: defer applySession(sessFrom(in.Tenant, in.Surface, in.Auth))()
	applySession := func(in sessionIn) (func(), error) {
		return buddy.BeginSessionOverrides(in.overrides())
	}

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_instructions",
		Description: descGetInstructions,
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		return toolOK(AgentPlaybook, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "buddy_info",
		Description: descBuddyInfo,
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		sel := buddy.SessionSelectionFor(buddy.ChannelMCP, "", credName)
		return toolOK(sel.InfoMap(version), nil)
	})

	type docsIn struct {
		Action string `json:"action,omitempty" jsonschema:"list (default) or get"`
		Query  string `json:"query,omitempty" jsonschema:"for list: case-insensitive fuzzy match on id/title/section/keywords; splits on _ - / spaces (e.g. smart_groups, smart groups, policy)"`
		ID     string `json:"id,omitempty" jsonschema:"for get: page id (e.g. docs/policies, builtins, docs/apps) or buddy://docs/… URI"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "docs",
		Description: descDocs,
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in docsIn) (*mcpsdk.CallToolResult, any, error) {
		pages, err := loadAgentDocs()
		if err != nil {
			return toolErr(err)
		}
		action := strings.ToLower(strings.TrimSpace(in.Action))
		if action == "" {
			action = "list"
		}
		switch action {
		case "list":
			entries := docsList(pages, in.Query)
			n := len(entries)
			return toolOK(entries, &n)
		case "get":
			page, err := docsGet(pages, in.ID)
			if err != nil {
				return toolErr(err)
			}
			return toolOK(page, nil)
		default:
			return toolErr(fmt.Errorf("unknown action %q (want list or get)", in.Action))
		}
	})

	type schemaIn struct {
		API     string `json:"api,omitempty" jsonschema:"pro (OpenAPI /api/schema), classic (Swagger /classicapi/doc/swagger.yaml), platform (embedded Platform API Gateway specs), both (pro+classic), or all (default; pro+classic+platform)"`
		Filter  string `json:"filter,omitempty" jsonschema:"case-insensitive; space-separated terms AND-matched against path, method, tags, summary, operationId, description"`
		Full    bool   `json:"full,omitempty" jsonschema:"if true, include filtered schema document(s) (large); default is operations catalog only"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "jamf_schema",
		Description: descJamfSchema,
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in schemaIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessFrom(in.Tenant, in.Surface, in.Auth))
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		api := in.API
		if api == "" {
			api = "all"
		}
		res, err := buddy.FetchJamfSchema(resolve(sessFrom(in.Tenant, in.Surface, in.Auth)), api, in.Filter, in.Full)
		if err != nil {
			return toolErr(err)
		}
		n := res.Count
		return toolOK(res, &n)
	})

	type evalIn struct {
		Source  string `json:"source" jsonschema:"Buddy Starlark source to evaluate (.buddy dialect / named builtins)"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "buddyscript",
		Description: descBuddyscript,
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in evalIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessFrom(in.Tenant, in.Surface, in.Auth))
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		sess, err := script.NewSession(resolve(sessFrom(in.Tenant, in.Surface, in.Auth)), opts, logger, false)
		if err != nil {
			return toolErr(err)
		}
		res := sess.EvalOpts("mcp", in.Source, script.EvalOptions{CallMain: true})
		stdout := res.Stdout
		if stdout == "" && res.Result != "" {
			stdout = res.Result
		}
		out := map[string]any{
			"stdout": stdout,
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
	// Agent catalog (generated agent-tone pages + examples/help), embedded by default.
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
		server.AddResource(&mcpsdk.Resource{
			URI:         uri,
			Name:        page.ID,
			Description: page.Title,
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
				Description: page.Title,
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
