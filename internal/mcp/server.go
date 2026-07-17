package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"buddy/internal/buddy"
	"buddy/internal/help"
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
		return toolOK(summarizeAuth(info), nil)
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
		Name:        "group_list_smart",
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
		Name:        "group_list_static",
		Description: "List Jamf static computer groups. Returns {ok, value: [{id,name,description},...], count}. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in filterIn) (*mcpsdk.CallToolResult, any, error) {
		groups, err := buddy.GroupSearchStatic(credFromFilter(in), in.Filter, pageOpts(in), logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(groups)
		return toolOK(groups, &n)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "computer_find_ids",
		Description: "Find computer IDs via RSQL filter. Returns {ok, value: [id,...], count}. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in filterIn) (*mcpsdk.CallToolResult, any, error) {
		recs, err := buddy.ComputerSearch(credFromFilter(in), in.Filter, pageOpts(in), logger)
		if err != nil {
			return toolErr(err)
		}
		ids := make([]int, 0, len(recs))
		for _, r := range recs {
			ids = append(ids, r.ID)
		}
		n := len(ids)
		return toolOK(ids, &n)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "script_list",
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
		Name:        "package_list",
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
		Name:        "policy_get_id",
		Description: "Resolve a Jamf policy ID from its trigger name. Returns {ok, value: {id, trigger}}.",
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

func summarizeAuth(info map[string]interface{}) map[string]any {
	out := map[string]any{}
	if at, ok := info["authenticationType"]; ok {
		out["authenticationType"] = at
	}
	acct, _ := info["account"].(map[string]interface{})
	if acct != nil {
		out["username"] = acct["username"]
		out["accessLevel"] = acct["accessLevel"]
		out["privilegeSet"] = acct["privilegeSet"]
		if bySite, ok := acct["privilegesBySite"].(map[string]interface{}); ok {
			n := 0
			for _, v := range bySite {
				if list, ok := v.([]interface{}); ok {
					n += len(list)
				}
			}
			out["privilegeCount"] = n
		}
	}
	if sites, ok := info["sites"]; ok {
		out["sites"] = sites
	}
	return out
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
	for _, p := range help.CLIPages() {
		page := p
		uri := "buddy://docs/help/" + page.ID
		server.AddResource(&mcpsdk.Resource{
			URI:         uri,
			Name:        "help/" + page.ID,
			Description: page.Title,
			MIMEType:    "text/plain",
		}, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
			return &mcpsdk.ReadResourceResult{
				Contents: []*mcpsdk.ResourceContents{{
					URI:      uri,
					MIMEType: "text/plain",
					Text:     page.Body,
				}},
			}, nil
		})
	}

	// Browsable builtins markdown (same as buddy serve /docs/builtins).
	uriBuiltins := "buddy://docs/builtins"
	builtinsMD := help.BuiltinsMarkdown()
	server.AddResource(&mcpsdk.Resource{
		URI:         uriBuiltins,
		Name:        "builtins",
		Description: "Starlark builtins reference (Markdown)",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{{
				URI:      uriBuiltins,
				MIMEType: "text/markdown",
				Text:     builtinsMD,
			}},
		}, nil
	})

	docsRoot, err := findDocsRoot()
	if err != nil {
		return
	}
	for _, name := range []string{"index.md", "reference-manual.md", "api-endpoints.md", "mcp.md"} {
		path := filepath.Join(docsRoot, name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		uri := "buddy://docs/" + name
		body := string(b)
		server.AddResource(&mcpsdk.Resource{
			URI:         uri,
			Name:        name,
			Description: "Buddy documentation: " + name,
			MIMEType:    "text/markdown",
		}, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
			return &mcpsdk.ReadResourceResult{
				Contents: []*mcpsdk.ResourceContents{{
					URI:      uri,
					MIMEType: "text/markdown",
					Text:     body,
				}},
			}, nil
		})
	}
}

func findDocsRoot() (string, error) {
	if env := os.Getenv("BUDDY_DOCS_ROOT"); env != "" {
		return env, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "docs")
		if fi, err := os.Stat(filepath.Join(candidate, "index.md")); err == nil && !fi.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("docs not found")
		}
		dir = parent
	}
}
