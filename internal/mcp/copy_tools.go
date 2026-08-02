package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"buddy/internal/buddy"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerCategoryScriptCopyTools(server *mcpsdk.Server, defaultAuth string, mutOpts func() buddy.Opts, logger *slog.Logger) {
	resolve := func(in sessionIn) string {
		if in.Auth != "" {
			return in.Auth
		}
		if in.Tenant != "" {
			return ""
		}
		return defaultAuth
	}
	applySession := func(in sessionIn) (func(), error) {
		return buddy.BeginSessionOverrides(in.overrides())
	}

	type categoriesIn struct {
		Filter   string `json:"filter,omitempty" jsonschema:"name substring or RSQL filter"`
		Page     *int   `json:"page,omitempty" jsonschema:"0-based page; omit to fetch all pages"`
		PageSize int    `json:"page_size,omitempty" jsonschema:"page size (default 200)"`
		Tenant   string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface  string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth     string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "categories",
		Description: "List Jamf Pro categories (summaries). Returns {ok, value: [{id,name,priority},...], count}. " +
			"Omit page to fetch all pages. Clone/copy/migrate with category_clone / category_copy / category_migrate.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in categoriesIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		o := buddy.PageOpts{Page: buddy.PageAll, PageSize: in.PageSize}
		if in.Page != nil {
			o.Page = *in.Page
		}
		cats, err := buddy.CategorySearch(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), in.Filter, o, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(cats)
		return toolOK(cats, &n)
	})

	type categoryCloneIn struct {
		CategoryID string `json:"category_id" jsonschema:"source category id"`
		Name       string `json:"name" jsonschema:"name for the new category"`
		Priority   *int   `json:"priority,omitempty" jsonschema:"optional priority override"`
		Tenant     string `json:"tenant,omitempty" jsonschema:"tenant override"`
		Surface    string `json:"surface,omitempty" jsonschema:"surface bias"`
		Auth       string `json:"auth,omitempty" jsonschema:"auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "category_clone",
		Description: "Clone a Jamf Pro category in the current session (mutating). " +
			"Returns {ok, value: {id, href?}}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in categoryCloneIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		out, err := buddy.CategoryClone(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), in.CategoryID, in.Name, in.Priority, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("category_clone: cancelled"))
		}
		return toolOK(out, nil)
	})

	type categoryCopyIn struct {
		CategoryID  string `json:"category_id" jsonschema:"source category id"`
		Name        string `json:"name" jsonschema:"name for the new category on destination"`
		Priority    *int   `json:"priority,omitempty" jsonschema:"optional priority override"`
		FromTenant  string `json:"from_tenant,omitempty" jsonschema:"source tenant override"`
		FromSurface string `json:"from_surface,omitempty" jsonschema:"source surface bias"`
		FromAuth    string `json:"from_auth,omitempty" jsonschema:"source auth pin"`
		Tenant      string `json:"tenant,omitempty" jsonschema:"destination tenant override"`
		Surface     string `json:"surface,omitempty" jsonschema:"destination surface bias"`
		Auth        string `json:"auth,omitempty" jsonschema:"destination auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "category_copy",
		Description: "Copy a Jamf Pro category across sessions (mutating). " +
			"Returns {ok, value: {id, href?}}. Prefer category_migrate to upsert by name.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in categoryCopyIn) (*mcpsdk.CallToolResult, any, error) {
		endFrom, sessErr := applySession(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		src := resolve(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		endFrom()
		endDest, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endDest()
		dst := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		out, err := buddy.CategoryCopy(src, dst, mutOpts(), buddy.CategoryCopySpec{
			SourceCategoryID: in.CategoryID,
			Name:             in.Name,
			Priority:         in.Priority,
		}, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("category_copy: cancelled"))
		}
		return toolOK(out, nil)
	})

	type categoryMigrateIn struct {
		CategoryID  string `json:"category_id" jsonschema:"source category id"`
		Name        string `json:"name,omitempty" jsonschema:"destination name (default: source name); used for name-match upsert"`
		Priority    *int   `json:"priority,omitempty" jsonschema:"optional priority override"`
		FromTenant  string `json:"from_tenant,omitempty" jsonschema:"source tenant override"`
		FromSurface string `json:"from_surface,omitempty" jsonschema:"source surface bias"`
		FromAuth    string `json:"from_auth,omitempty" jsonschema:"source auth pin"`
		Tenant      string `json:"tenant,omitempty" jsonschema:"destination tenant override"`
		Surface     string `json:"surface,omitempty" jsonschema:"destination surface bias"`
		Auth        string `json:"auth,omitempty" jsonschema:"destination auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "category_migrate",
		Description: "Migrate/upsert a Jamf Pro category across sessions by exact name (mutating). " +
			"Returns {ok, value: {id, name, priority, created}}. Migrate categories before scripts/policies that reference them.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in categoryMigrateIn) (*mcpsdk.CallToolResult, any, error) {
		endFrom, sessErr := applySession(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		src := resolve(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		endFrom()
		endDest, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endDest()
		dst := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		out, err := buddy.CategoryMigrate(src, dst, mutOpts(), buddy.CategoryMigrateSpec{
			SourceCategoryID: in.CategoryID,
			Name:             in.Name,
			Priority:         in.Priority,
		}, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("category_migrate: cancelled"))
		}
		return toolOK(out, nil)
	})

	type scriptCloneIn struct {
		ScriptID   string `json:"script_id" jsonschema:"source script id"`
		Name       string `json:"name" jsonschema:"name for the new script"`
		CategoryID string `json:"category_id,omitempty" jsonschema:"optional destination category id override"`
		Tenant     string `json:"tenant,omitempty" jsonschema:"tenant override"`
		Surface    string `json:"surface,omitempty" jsonschema:"surface bias"`
		Auth       string `json:"auth,omitempty" jsonschema:"auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "script_clone",
		Description: "Clone a Jamf Pro script in the current session (mutating). " +
			"Keeps source category unless category_id set. Returns {ok, value: {id, name}}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in scriptCloneIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		out, err := buddy.ScriptClone(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), in.ScriptID, in.Name, in.CategoryID, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("script_clone: cancelled"))
		}
		return toolOK(map[string]any{"id": out.ID, "name": out.Name}, nil)
	})

	type scriptCopyIn struct {
		ScriptID    string `json:"script_id" jsonschema:"source script id"`
		Name        string `json:"name" jsonschema:"name for the new script on destination"`
		CategoryID  string `json:"category_id,omitempty" jsonschema:"optional destination category id override"`
		FromTenant  string `json:"from_tenant,omitempty" jsonschema:"source tenant override"`
		FromSurface string `json:"from_surface,omitempty" jsonschema:"source surface bias"`
		FromAuth    string `json:"from_auth,omitempty" jsonschema:"source auth pin"`
		Tenant      string `json:"tenant,omitempty" jsonschema:"destination tenant override"`
		Surface     string `json:"surface,omitempty" jsonschema:"destination surface bias"`
		Auth        string `json:"auth,omitempty" jsonschema:"destination auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "script_copy",
		Description: "Copy a Jamf Pro script across sessions (mutating). " +
			"Resolves category by name on destination (soft: missing → uncategorized) unless category_id set. " +
			"Returns {ok, value: {id, name}}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in scriptCopyIn) (*mcpsdk.CallToolResult, any, error) {
		endFrom, sessErr := applySession(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		src := resolve(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		endFrom()
		endDest, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endDest()
		dst := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		out, err := buddy.ScriptCopy(src, dst, mutOpts(), buddy.ScriptCopySpec{
			SourceScriptID: in.ScriptID,
			Name:           in.Name,
			CategoryID:     in.CategoryID,
		}, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("script_copy: cancelled"))
		}
		return toolOK(map[string]any{"id": out.ID, "name": out.Name}, nil)
	})

	type scriptMigrateIn struct {
		ScriptID    string `json:"script_id" jsonschema:"source script id"`
		Name        string `json:"name,omitempty" jsonschema:"destination name (default: source name); used for name-match upsert"`
		CategoryID  string `json:"category_id,omitempty" jsonschema:"optional destination category id override"`
		FromTenant  string `json:"from_tenant,omitempty" jsonschema:"source tenant override"`
		FromSurface string `json:"from_surface,omitempty" jsonschema:"source surface bias"`
		FromAuth    string `json:"from_auth,omitempty" jsonschema:"source auth pin"`
		Tenant      string `json:"tenant,omitempty" jsonschema:"destination tenant override"`
		Surface     string `json:"surface,omitempty" jsonschema:"destination surface bias"`
		Auth        string `json:"auth,omitempty" jsonschema:"destination auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "script_migrate",
		Description: "Migrate/upsert a Jamf Pro script across sessions by exact name (mutating). " +
			"Returns {ok, value: {id, name, created}}. Migrate scripts before policy_copy when policies reference them.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in scriptMigrateIn) (*mcpsdk.CallToolResult, any, error) {
		endFrom, sessErr := applySession(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		src := resolve(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		endFrom()
		endDest, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endDest()
		dst := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		out, err := buddy.ScriptMigrate(src, dst, mutOpts(), buddy.ScriptMigrateSpec{
			SourceScriptID: in.ScriptID,
			Name:           in.Name,
			CategoryID:     in.CategoryID,
		}, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("script_migrate: cancelled"))
		}
		return toolOK(map[string]any{"id": out.ID, "name": out.Name, "created": out.Created}, nil)
	})
}
