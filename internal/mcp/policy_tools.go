package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPolicyTools(server *mcpsdk.Server, defaultAuth string, mutOpts func() buddy.Opts, logger *slog.Logger) {
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

	type policiesIn struct {
		Keywords     []string `json:"keywords,omitempty" jsonschema:"name/trigger keywords; omit or empty to list all"`
		ExcludeIDs   []int    `json:"exclude_ids,omitempty" jsonschema:"policy ids to exclude"`
		ForceRefresh bool     `json:"force_refresh,omitempty" jsonschema:"bypass policy list cache"`
		IDs          bool     `json:"ids,omitempty" jsonschema:"if true, return list of int ids instead of {id,name}"`
		Tenant       string   `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface      string   `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth         string   `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "policies",
		Description: "List/search Jamf Classic policies (summaries). " +
			"Returns {ok, value: [{id,name},...], count}, or IDs if ids=true. " +
			"Omit keywords (or empty) to list all. For release workflows (policy_set_ss / policy_set_package / versioned policy_clone) use starlark_eval.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policiesIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		cred := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		summaries, err := buddy.PolicyFindSummariesByKeywords(cred, mutOpts(), in.Keywords, in.ExcludeIDs, in.ForceRefresh, logger)
		if err != nil {
			return toolErr(err)
		}
		if in.IDs {
			ids := make([]int, 0, len(summaries))
			for _, s := range summaries {
				ids = append(ids, s.ID)
			}
			n := len(ids)
			return toolOK(ids, &n)
		}
		out := make([]map[string]any, 0, len(summaries))
		for _, s := range summaries {
			out = append(out, map[string]any{"id": s.ID, "name": s.Name})
		}
		n := len(out)
		return toolOK(out, &n)
	})

	type policyIn struct {
		ID           *int   `json:"id,omitempty" jsonschema:"policy id (pass id or trigger, not both)"`
		Trigger      string `json:"trigger,omitempty" jsonschema:"custom trigger_other (pass id or trigger, not both)"`
		ForceRefresh bool   `json:"force_refresh,omitempty" jsonschema:"bypass cache when resolving by trigger"`
		Tenant       string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface      string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth         string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "policy",
		Description: "Load one Jamf Classic policy by id or custom trigger. " +
			"Returns {ok, value: details dict} (id, name, enabled, trigger, trigger_other, frequency, scope, packages, scripts). " +
			"Pass exactly one of id or trigger. For XML get/put use policy_xml / policy_put_xml; " +
			"for release helpers (policy_set_ss, policy_set_package, versioned policy_clone) use starlark_eval.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policyIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		cred := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		idSet := in.ID != nil
		trig := strings.TrimSpace(in.Trigger)
		keySet := trig != ""
		if idSet == keySet {
			return toolErr(fmt.Errorf("policy: pass exactly one of id or trigger"))
		}
		var pid int
		if idSet {
			pid = *in.ID
			if pid <= 0 {
				return toolErr(fmt.Errorf("policy: id must be positive"))
			}
		} else {
			id, err := buddy.PolicyFind(cred, mutOpts(), trig, in.ForceRefresh, logger)
			if err != nil {
				return toolErr(err)
			}
			pid = id
		}
		d, err := buddy.PolicyGetDetails(cred, pid, logger)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(buddy.PolicyDetailsMap(d), nil)
	})

	type policyXMLIn struct {
		ID      int    `json:"id" jsonschema:"policy id"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "policy_xml",
		Description: "Fetch raw Classic policy XML by id. Returns {ok, value: xml string}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policyXMLIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		xmlBytes, err := buddy.PolicyFetchXML(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), in.ID)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(string(xmlBytes), nil)
	})

	type policyPutXMLIn struct {
		PolicyID int    `json:"policy_id" jsonschema:"policy id to update"`
		XMLStr   string `json:"xml_str" jsonschema:"full Classic policy XML"`
		Tenant   string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface  string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth     string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "policy_put_xml",
		Description: "PUT Classic policy XML (mutating). Returns {ok, value: null}. Prefer policy_set for simple field edits.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policyPutXMLIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if err := buddy.PolicyPutXML(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), in.PolicyID, []byte(in.XMLStr), logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	type policySetIn struct {
		PolicyID  int    `json:"policy_id" jsonschema:"policy id"`
		Enable    *bool  `json:"enable,omitempty" jsonschema:"enable or disable the policy"`
		Name      string `json:"name,omitempty" jsonschema:"new policy name"`
		Trigger   string `json:"trigger,omitempty" jsonschema:"new custom trigger_other (policy must already have one)"`
		Frequency string `json:"frequency,omitempty" jsonschema:"e.g. Once per computer, Ongoing"`
		Tenant    string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface   string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth      string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "policy_set",
		Description: "Update basic Classic policy fields (mutating): enable, name, trigger (trigger_other), and/or frequency. " +
			"Pass at least one field. Returns {ok, value: null}. " +
			"For Self Service / package release helpers use starlark_eval (policy_set_ss, policy_set_package, …).",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policySetIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if in.Enable == nil && strings.TrimSpace(in.Name) == "" && strings.TrimSpace(in.Trigger) == "" && strings.TrimSpace(in.Frequency) == "" {
			return toolErr(fmt.Errorf("policy_set: pass at least one of enable, name, trigger, frequency"))
		}
		cred := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		o := mutOpts()
		if in.Enable != nil {
			if err := buddy.PolicyEnable(cred, o, in.PolicyID, *in.Enable, logger); err != nil {
				return toolErr(err)
			}
		}
		if name := strings.TrimSpace(in.Name); name != "" {
			if err := buddy.PolicySetName(cred, o, in.PolicyID, name, logger); err != nil {
				return toolErr(err)
			}
		}
		if trig := strings.TrimSpace(in.Trigger); trig != "" {
			if err := buddy.PolicyTrigger(cred, o, in.PolicyID, trig, logger); err != nil {
				return toolErr(err)
			}
		}
		if freq := strings.TrimSpace(in.Frequency); freq != "" {
			if err := buddy.PolicySetFrequency(cred, o, in.PolicyID, freq, logger); err != nil {
				return toolErr(err)
			}
		}
		return toolOK(nil, nil)
	})

	type policyDeleteIn struct {
		PolicyIDs []int  `json:"policy_ids" jsonschema:"policy ids to delete"`
		Tenant    string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface   string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth      string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "policy_delete",
		Description: "Delete one or more Classic policies by id (mutating). Returns {ok, value: null}.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policyDeleteIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if len(in.PolicyIDs) == 0 {
			return toolErr(fmt.Errorf("policy_delete: policy_ids is required"))
		}
		if err := buddy.PolicyDelete(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), in.PolicyIDs, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	type policyCopyIn struct {
		PolicyID      int    `json:"policy_id" jsonschema:"source policy id"`
		Name          string `json:"name" jsonschema:"name for the new policy on destination"`
		Trigger       string `json:"trigger,omitempty" jsonschema:"optional new custom trigger_other; omit to keep source"`
		CategoryID    string `json:"category_id,omitempty" jsonschema:"optional destination category id override"`
		FromTenant    string `json:"from_tenant,omitempty" jsonschema:"source tenant override"`
		FromSurface   string `json:"from_surface,omitempty" jsonschema:"source surface bias"`
		FromAuth      string `json:"from_auth,omitempty" jsonschema:"source auth pin"`
		ScopeAll      *bool  `json:"scope_all,omitempty" jsonschema:"destination scope all computers (default unassigned)"`
		ScopeGroupIDs []int  `json:"scope_group_ids,omitempty" jsonschema:"destination computer group ids"`
		Tenant        string `json:"tenant,omitempty" jsonschema:"destination tenant override"`
		Surface       string `json:"surface,omitempty" jsonschema:"destination surface bias"`
		Auth          string `json:"auth,omitempty" jsonschema:"destination auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "policy_copy",
		Description: "Copy a Classic policy across sessions (mutating). " +
			"Prepares XML (strip id, set name/trigger, default unassigned scope), remaps category/scripts/packages by name on destination " +
			"(missing scripts/packages fail — migrate them first; missing category → uncategorized), POSTs on destination. " +
			"Returns {ok, value: {id, name, trigger}}. Not versioned policy_clone (release helper via starlark_eval).",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policyCopyIn) (*mcpsdk.CallToolResult, any, error) {
		endFrom, sessErr := applySession(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		srcCred := resolve(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		endFrom()
		endDest, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endDest()
		dstCred := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		spec := buddy.PolicyCopySpec{
			SourcePolicyID: in.PolicyID,
			Name:           in.Name,
			ScopeAll:       in.ScopeAll,
			ScopeGroupIDs:  in.ScopeGroupIDs,
			CategoryID:     in.CategoryID,
		}
		if t := strings.TrimSpace(in.Trigger); t != "" {
			spec.Trigger = &t
		}
		created, err := buddy.PolicyCopy(srcCred, dstCred, mutOpts(), spec, logger)
		if err != nil {
			return toolErr(err)
		}
		if created == nil {
			return toolErr(fmt.Errorf("policy_copy: cancelled"))
		}
		return toolOK(map[string]any{"id": created.PolicyID, "name": created.Name, "trigger": created.Trigger}, nil)
	})

	type policyMigrateIn struct {
		PolicyID      int    `json:"policy_id" jsonschema:"source policy id"`
		Name          string `json:"name,omitempty" jsonschema:"destination name (default: source name); also used for name-match upsert"`
		Trigger       string `json:"trigger,omitempty" jsonschema:"optional new custom trigger_other; omit to keep source"`
		MatchTrigger  bool   `json:"match_trigger,omitempty" jsonschema:"if true, find destination by trigger_other instead of exact name"`
		CategoryID    string `json:"category_id,omitempty" jsonschema:"optional destination category id override"`
		FromTenant    string `json:"from_tenant,omitempty" jsonschema:"source tenant override"`
		FromSurface   string `json:"from_surface,omitempty" jsonschema:"source surface bias"`
		FromAuth      string `json:"from_auth,omitempty" jsonschema:"source auth pin"`
		ScopeAll      *bool  `json:"scope_all,omitempty" jsonschema:"destination scope all computers (default unassigned)"`
		ScopeGroupIDs []int  `json:"scope_group_ids,omitempty" jsonschema:"destination computer group ids"`
		Tenant        string `json:"tenant,omitempty" jsonschema:"destination tenant override"`
		Surface       string `json:"surface,omitempty" jsonschema:"destination surface bias"`
		Auth          string `json:"auth,omitempty" jsonschema:"destination auth pin"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "policy_migrate",
		Description: "Migrate/upsert a Classic policy across sessions (mutating). " +
			"Same prepare/remap as policy_copy; on destination if a policy with the same name exists (or trigger_other when match_trigger=true), PUT preserving that id; else POST. " +
			"Returns {ok, value: {id, name, trigger, created}}. Recommended order: category_migrate → script_migrate → policy_migrate.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in policyMigrateIn) (*mcpsdk.CallToolResult, any, error) {
		endFrom, sessErr := applySession(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		srcCred := resolve(sessionIn{Tenant: in.FromTenant, Surface: in.FromSurface, Auth: in.FromAuth})
		endFrom()
		endDest, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endDest()
		dstCred := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		spec := buddy.PolicyMigrateSpec{
			SourcePolicyID: in.PolicyID,
			Name:           in.Name,
			MatchTrigger:   in.MatchTrigger,
			ScopeAll:       in.ScopeAll,
			ScopeGroupIDs:  in.ScopeGroupIDs,
			CategoryID:     in.CategoryID,
		}
		if t := strings.TrimSpace(in.Trigger); t != "" {
			spec.Trigger = &t
		}
		out, err := buddy.PolicyMigrate(srcCred, dstCred, mutOpts(), spec, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("policy_migrate: cancelled"))
		}
		return toolOK(map[string]any{
			"id": out.PolicyID, "name": out.Name, "trigger": out.Trigger, "created": out.Created,
		}, nil)
	})
}
