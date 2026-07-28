package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
	registerTools(server, credName, opts, logger, version)
	registerDocResources(server)
	return server.Run(context.Background(), &mcpsdk.StdioTransport{})
}

func registerTools(server *mcpsdk.Server, credName string, opts buddy.Opts, logger *slog.Logger, version string) {
	type credIn struct {
		Cred string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	resolve := func(in credIn) string {
		if in.Cred != "" {
			return in.Cred
		}
		return credName
	}

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "buddy_info",
		Description: "Buddy binary version and MCP session selection: default tenant, optional auth pin, " +
			"and which tenants/auths are available on the mcp channel (respects auth channels:). " +
			"Does not call Jamf. Use auth_test to verify credentials against the API.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		sel := buddy.SessionSelectionFor(buddy.ChannelMCP, "", credName)
		return toolOK(sel.InfoMap(version), nil)
	})

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

	type packageIDIn struct {
		ID   string `json:"id" jsonschema:"package id"`
		Cred string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "package_ready",
		Description: "Check whether a Jamf package exists and cloudTransferStatus is READY " +
			"(JCDS available; UI Availability Pending cleared). Returns {ok, value: bool}. " +
			"Missing package → value false (not an error).",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in packageIDIn) (*mcpsdk.CallToolResult, any, error) {
		ready, err := buddy.PackageReady(resolve(credIn{Cred: in.Cred}), strings.TrimSpace(in.ID))
		if err != nil {
			return toolErr(err)
		}
		return toolOK(ready, nil)
	})

	type packageWaitReadyIn struct {
		ID      string `json:"id" jsonschema:"package id"`
		Timeout *int   `json:"timeout,omitempty" jsonschema:"timeout in seconds (default 600)"`
		Cred    string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "package_wait_ready",
		Description: "Poll until a Jamf package has cloudTransferStatus READY (after package_upload / before policy attach). " +
			"Returns {ok, value: null}. Default timeout 600s. Nudges JCDS refresh-inventory while waiting.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in packageWaitReadyIn) (*mcpsdk.CallToolResult, any, error) {
		var d time.Duration
		if in.Timeout != nil {
			if *in.Timeout < 0 {
				return toolErr(fmt.Errorf("timeout must be >= 0"))
			}
			d = time.Duration(*in.Timeout) * time.Second
		}
		if err := buddy.PackageWaitReady(resolve(credIn{Cred: in.Cred}), strings.TrimSpace(in.ID), d, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	type packageDownloadIn struct {
		ID   string `json:"id" jsonschema:"package id"`
		Path string `json:"path,omitempty" jsonschema:"local outfile path (default: package fileName in cwd)"`
		Cred string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "package_download",
		Description: "Download a Jamf package file via JCDS (Jamf Cloud only). Waits for READY+sha256, " +
			"streams a pre-signed URI to a temp file while hashing SHA-256, renames only on match. " +
			"Returns {ok, value: null}. Writes to the MCP server host filesystem.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in packageDownloadIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.PackageDownload(resolve(credIn{Cred: in.Cred}), strings.TrimSpace(in.ID), strings.TrimSpace(in.Path), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
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

	// Blueprints (Platform API Gateway credential required: region/url + tenantId).
	mutOpts := func() buddy.Opts {
		o := opts
		o.SkipConfirm = true // no interactive stdin in MCP; the host approves the tool call
		return o
	}

	type blueprintListIn struct {
		Search   string `json:"search,omitempty" jsonschema:"search name/description"`
		Sort     string `json:"sort,omitempty" jsonschema:"sort field:dir (default created:desc)"`
		Page     *int   `json:"page,omitempty" jsonschema:"0-based page; omit to fetch all pages"`
		PageSize int    `json:"page_size,omitempty" jsonschema:"page size (default 200)"`
		Cred     string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprints",
		Description: "List Jamf Platform API blueprints (Platform gateway credential required). Returns {ok, value:[{id,name,description,...}], count}. Omit page to fetch all pages.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintListIn) (*mcpsdk.CallToolResult, any, error) {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		items, err := buddy.BlueprintListValues(resolve(credIn{Cred: in.Cred}), buddy.BlueprintListOpts{Search: in.Search, Sort: in.Sort, Page: o}, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(items)
		return toolOK(items, &n)
	})

	type blueprintIDIn struct {
		BlueprintID string `json:"blueprint_id" jsonschema:"blueprint UUID"`
		Cred        string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint",
		Description: "Load one Jamf Platform API blueprint by id (Platform gateway credential required). Returns {ok, value: full blueprint JSON}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintIDIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.BlueprintGetValue(resolve(credIn{Cred: in.Cred}), in.BlueprintID, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint_status",
		Description: "Fetch a blueprint's deployment status report (Platform gateway credential required). Returns {ok, value: report JSON}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintIDIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.BlueprintStatusValue(resolve(credIn{Cred: in.Cred}), in.BlueprintID, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})

	type blueprintComponentsIn struct {
		Identifier string `json:"identifier,omitempty" jsonschema:"component identifier; omit to list all components"`
		Page       *int   `json:"page,omitempty" jsonschema:"0-based page; omit to fetch all pages"`
		PageSize   int    `json:"page_size,omitempty" jsonschema:"page size (default 200)"`
		Cred       string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint_components",
		Description: "List blueprint components, or load one when identifier is set (Platform gateway credential required). Returns {ok, value: [...]|{...}, count?}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintComponentsIn) (*mcpsdk.CallToolResult, any, error) {
		cred := resolve(credIn{Cred: in.Cred})
		if strings.TrimSpace(in.Identifier) != "" {
			out, err := buddy.BlueprintComponentValue(cred, in.Identifier, logger)
			if err != nil {
				return toolErr(err)
			}
			return toolOK(out, nil)
		}
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		items, err := buddy.BlueprintComponentsValues(cred, o, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(items)
		return toolOK(items, &n)
	})

	type blueprintCreateIn struct {
		Body map[string]any `json:"body" jsonschema:"blueprint JSON body (object)"`
		Cred string         `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint_create",
		Description: "Create a Jamf Platform API blueprint from a JSON body (Platform gateway credential required; mutating). Returns {ok, value: create response}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintCreateIn) (*mcpsdk.CallToolResult, any, error) {
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		out, err := buddy.BlueprintCreateValue(resolve(credIn{Cred: in.Cred}), mutOpts(), bodyJSON, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})

	type blueprintSetIn struct {
		BlueprintID string         `json:"blueprint_id" jsonschema:"blueprint UUID"`
		Body        map[string]any `json:"body" jsonschema:"merge-patch JSON body (object)"`
		Cred        string         `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint_set",
		Description: "Update a blueprint via merge-patch JSON (Platform gateway credential required; mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintSetIn) (*mcpsdk.CallToolResult, any, error) {
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		if err := buddy.BlueprintSetValue(resolve(credIn{Cred: in.Cred}), in.BlueprintID, mutOpts(), bodyJSON, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.BlueprintID}, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint_delete",
		Description: "Delete a blueprint by id (Platform gateway credential required; mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintIDIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.BlueprintDeleteValue(resolve(credIn{Cred: in.Cred}), in.BlueprintID, mutOpts(), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.BlueprintID}, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint_deploy",
		Description: "Deploy a blueprint by id (Platform gateway credential required; mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintIDIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.BlueprintDeploy(resolve(credIn{Cred: in.Cred}), in.BlueprintID, mutOpts(), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.BlueprintID, "action": "deploy"}, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "blueprint_undeploy",
		Description: "Undeploy a blueprint by id (Platform gateway credential required; mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in blueprintIDIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.BlueprintUndeploy(resolve(credIn{Cred: in.Cred}), in.BlueprintID, mutOpts(), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.BlueprintID, "action": "undeploy"}, nil)
	})

	// Platform devices / device-groups / ddm / benchmarks
	type deviceListIn struct {
		Filter   string `json:"filter,omitempty" jsonschema:"RSQL filter"`
		Sort     string `json:"sort,omitempty" jsonschema:"sort field:dir"`
		Page     *int   `json:"page,omitempty" jsonschema:"0-based page; omit to fetch all pages"`
		PageSize int    `json:"page_size,omitempty" jsonschema:"page size (default 200)"`
		Cred     string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "devices",
		Description: "List Jamf Platform API devices (gateway credential required; distinct from Pro computers). Returns {ok, value:[{id,name,...}], count}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceListIn) (*mcpsdk.CallToolResult, any, error) {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		items, err := buddy.DeviceListValues(resolve(credIn{Cred: in.Cred}), buddy.DeviceListOpts{Filter: in.Filter, Sort: in.Sort, Page: o}, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(items)
		return toolOK(items, &n)
	})
	type deviceIDIn struct {
		DeviceID string `json:"device_id" jsonschema:"Platform device UUID"`
		Cred     string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device",
		Description: "Load one Platform API device by id. Returns {ok, value: device JSON}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceIDIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.DeviceGetValue(resolve(credIn{Cred: in.Cred}), in.DeviceID, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	type deviceSetIn struct {
		DeviceID string         `json:"device_id" jsonschema:"Platform device UUID"`
		Body     map[string]any `json:"body" jsonschema:"PATCH JSON body"`
		Cred     string         `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_set",
		Description: "PATCH a Platform device (mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceSetIn) (*mcpsdk.CallToolResult, any, error) {
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		if err := buddy.DeviceSetValue(resolve(credIn{Cred: in.Cred}), in.DeviceID, mutOpts(), bodyJSON, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.DeviceID}, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_delete",
		Description: "Delete a Platform device (mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceIDIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.DeviceDeleteValue(resolve(credIn{Cred: in.Cred}), in.DeviceID, mutOpts(), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.DeviceID}, nil)
	})
	type deviceAppsIn struct {
		DeviceID string `json:"device_id" jsonschema:"Platform device UUID"`
		Filter   string `json:"filter,omitempty"`
		Page     *int   `json:"page,omitempty"`
		PageSize int    `json:"page_size,omitempty"`
		Cred     string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_applications",
		Description: "List installed applications for a Platform device. Returns {ok, value:[...], count}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceAppsIn) (*mcpsdk.CallToolResult, any, error) {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		items, err := buddy.DeviceApplicationsValue(resolve(credIn{Cred: in.Cred}), in.DeviceID, o, in.Filter, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(items)
		return toolOK(items, &n)
	})
	for _, act := range []struct {
		name, desc string
		fn         func(string, string, buddy.Opts, *slog.Logger) error
	}{
		{"device_check_in", "Request Platform device check-in (mutating).", buddy.DeviceCheckIn},
		{"device_restart", "Restart a Platform device (mutating).", buddy.DeviceRestart},
		{"device_shutdown", "Shut down a Platform device (mutating).", buddy.DeviceShutdown},
		{"device_erase", "Erase a Platform device (mutating).", buddy.DeviceErase},
		{"device_unmanage", "Unmanage a Platform device (mutating).", buddy.DeviceUnmanage},
	} {
		act := act
		mcpsdk.AddTool(server, &mcpsdk.Tool{
			Name:        act.name,
			Description: act.desc + " Returns {ok}.",
		}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceIDIn) (*mcpsdk.CallToolResult, any, error) {
			if err := act.fn(resolve(credIn{Cred: in.Cred}), in.DeviceID, mutOpts(), logger); err != nil {
				return toolErr(err)
			}
			return toolOK(map[string]any{"id": in.DeviceID, "action": act.name}, nil)
		})
	}

	type deviceGroupListIn struct {
		Filter   string `json:"filter,omitempty" jsonschema:"RSQL filter"`
		Page     *int   `json:"page,omitempty"`
		PageSize int    `json:"page_size,omitempty"`
		Cred     string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_groups",
		Description: "List Platform API device groups (not Pro computer groups). Returns {ok, value:[...], count}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupListIn) (*mcpsdk.CallToolResult, any, error) {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		items, err := buddy.DeviceGroupListValues(resolve(credIn{Cred: in.Cred}), buddy.DeviceGroupListOpts{Filter: in.Filter, Page: o}, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(items)
		return toolOK(items, &n)
	})
	type deviceGroupIDIn struct {
		GroupID string `json:"group_id" jsonschema:"Platform device group UUID"`
		Cred    string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_group",
		Description: "Load one Platform device group by id. Returns {ok, value: group JSON}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupIDIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.DeviceGroupGetValue(resolve(credIn{Cred: in.Cred}), in.GroupID, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	type deviceGroupBodyIn struct {
		Body map[string]any `json:"body" jsonschema:"JSON body"`
		Cred string         `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_group_create",
		Description: "Create a Platform device group (mutating). Returns {ok, value: create response}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupBodyIn) (*mcpsdk.CallToolResult, any, error) {
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		out, err := buddy.DeviceGroupCreateValue(resolve(credIn{Cred: in.Cred}), mutOpts(), bodyJSON, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	type deviceGroupSetIn struct {
		GroupID string         `json:"group_id"`
		Body    map[string]any `json:"body"`
		Cred    string         `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_group_set",
		Description: "PATCH a Platform device group (mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupSetIn) (*mcpsdk.CallToolResult, any, error) {
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		if err := buddy.DeviceGroupSetValue(resolve(credIn{Cred: in.Cred}), in.GroupID, mutOpts(), bodyJSON, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.GroupID}, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_group_delete",
		Description: "Delete a Platform device group (mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupIDIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.DeviceGroupDeleteValue(resolve(credIn{Cred: in.Cred}), in.GroupID, mutOpts(), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.GroupID}, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_group_members",
		Description: "List members of a Platform device group. Returns {ok, value: members JSON}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupIDIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.DeviceGroupMembersValue(resolve(credIn{Cred: in.Cred}), in.GroupID, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "device_group_set_members",
		Description: "PATCH membership of a static Platform device group (mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupSetIn) (*mcpsdk.CallToolResult, any, error) {
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		if err := buddy.DeviceGroupSetMembersValue(resolve(credIn{Cred: in.Cred}), in.GroupID, mutOpts(), bodyJSON, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.GroupID}, nil)
	})

	type ddmDeclIn struct {
		DeclarationID string `json:"declaration_id" jsonschema:"declaration identifier"`
		Filter        string `json:"filter,omitempty"`
		Sort          string `json:"sort,omitempty"`
		Page          *int   `json:"page,omitempty"`
		PageSize      int    `json:"page_size,omitempty"`
		Cred          string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "ddm_declaration_devices",
		Description: "List devices reporting a DDM declaration (Platform read-only). Returns {ok, value}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in ddmDeclIn) (*mcpsdk.CallToolResult, any, error) {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		out, err := buddy.DDMDeclarationDevicesValue(resolve(credIn{Cred: in.Cred}), in.DeclarationID, o, in.Filter, in.Sort, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	type ddmDeviceIn struct {
		DeviceID string `json:"device_id"`
		Filter   string `json:"filter,omitempty"`
		Sort     string `json:"sort,omitempty"`
		Page     *int   `json:"page,omitempty"`
		PageSize int    `json:"page_size,omitempty"`
		Cred     string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "ddm_device_declarations",
		Description: "List DDM declarations reported for a device. Returns {ok, value}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in ddmDeviceIn) (*mcpsdk.CallToolResult, any, error) {
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		out, err := buddy.DDMDeviceDeclarationsValue(resolve(credIn{Cred: in.Cred}), in.DeviceID, o, in.Filter, in.Sort, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "ddm_device_channels",
		Description: "List DDM channels for a device. Returns {ok, value}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceIDIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.DDMDeviceChannelsValue(resolve(credIn{Cred: in.Cred}), in.DeviceID, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})

	type credOnlyIn struct {
		Cred string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "benchmarks",
		Description: "List Platform compliance benchmarks (beta). Returns {ok, value}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in credOnlyIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.BenchmarkListValues(resolve(credIn{Cred: in.Cred}), logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	type benchmarkIDIn struct {
		BenchmarkID string `json:"benchmark_id"`
		Cred        string `json:"cred,omitempty"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "benchmark",
		Description: "Load one compliance benchmark by id (beta). Returns {ok, value}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in benchmarkIDIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.BenchmarkGetValue(resolve(credIn{Cred: in.Cred}), in.BenchmarkID, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "benchmark_create",
		Description: "Create a compliance benchmark (beta, mutating). Returns {ok, value}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in deviceGroupBodyIn) (*mcpsdk.CallToolResult, any, error) {
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		out, err := buddy.BenchmarkCreateValue(resolve(credIn{Cred: in.Cred}), mutOpts(), bodyJSON, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "benchmark_delete",
		Description: "Delete a compliance benchmark (beta, mutating). Returns {ok}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in benchmarkIDIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.BenchmarkDeleteValue(resolve(credIn{Cred: in.Cred}), in.BenchmarkID, mutOpts(), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.BenchmarkID}, nil)
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "benchmark_baselines",
		Description: "List mSCP baselines for compliance benchmarks (beta). Returns {ok, value}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in credOnlyIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.BenchmarkBaselinesValue(resolve(credIn{Cred: in.Cred}), logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})

	type schemaIn struct {
		API    string `json:"api,omitempty" jsonschema:"pro (OpenAPI /api/schema), classic (Swagger /classicapi/doc/swagger.yaml), platform (embedded Platform API Gateway specs), both (pro+classic), or all (default; pro+classic+platform)"`
		Filter string `json:"filter,omitempty" jsonschema:"case-insensitive; space-separated terms AND-matched against path, method, tags, summary, operationId, description"`
		Full   bool   `json:"full,omitempty" jsonschema:"if true, include filtered schema document(s) (large); default is operations catalog only"`
		Cred   string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "jamf_schema",
		Description: "Look up Jamf API operations from the server schemas: Jamf Pro OpenAPI (/api/schema) " +
			"and Classic Swagger (/classicapi/doc/swagger.yaml) fetched live, plus the Platform API Gateway " +
			"specs (blueprints, devices, device-groups, device-actions, ddm/report, compliance-benchmarks) " +
			"embedded in the binary. Default api=all searches Pro, Classic, and Platform so you can learn " +
			"where an endpoint lives (api=platform needs no credential). " +
			"Use filter (e.g. \"blueprints\" or \"policy delete\") to narrow results. " +
			"Returns {ok, value: {api, count, operations:[{api,path,method,summary,...}], pro?, classic?, platform?}}. " +
			"Set full=true only when you need response/parameter schemas.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in schemaIn) (*mcpsdk.CallToolResult, any, error) {
		api := in.API
		if api == "" {
			api = "all"
		}
		res, err := buddy.FetchJamfSchema(resolve(credIn{Cred: in.Cred}), api, in.Filter, in.Full)
		if err != nil {
			return toolErr(err)
		}
		n := res.Count
		return toolOK(res, &n)
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

	type docsIn struct {
		Action string `json:"action,omitempty" jsonschema:"list (default) or get"`
		Query  string `json:"query,omitempty" jsonschema:"for list: case-insensitive substring on id/title/section"`
		ID     string `json:"id,omitempty" jsonschema:"for get: page id (e.g. agents, builtins, rules/buddy-builtins) or buddy://docs/… URI"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "docs",
		Description: "Read buddy embedded documentation and design rules (same catalog as buddy://docs/… resources). " +
			"Prefer this before inventing buddy conventions. " +
			"action=list (default) returns [{id,title,section,uri},…]; optional query filters. " +
			"action=get requires id and returns {id,title,uri,markdown}. List then get.",
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
