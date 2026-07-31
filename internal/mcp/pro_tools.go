package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"buddy/internal/buddy"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerProComputerGroupTools(server *mcpsdk.Server, defaultAuth string, mutOpts func() buddy.Opts, logger *slog.Logger) {
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

	type smartGroupIn struct {
		ID   string `json:"id,omitempty" jsonschema:"smart group id (pass id or name, not both)"`
		Name string `json:"name,omitempty" jsonschema:"smart group name (pass id or name, not both)"`
		Tenant string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "smart_group",
		Description: "Load one Jamf Pro smart computer group (criteria-based membership) by id or name. " +
			"Returns {ok, value: full smart group JSON including criteria}. " +
			"Not a static group (explicit computer list) and not a Platform device_group. " +
			"Pass exactly one of id or name.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in smartGroupIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		cred := resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		groupID, err := buddy.SmartGroupResolveID(cred, in.ID, in.Name, logger)
		if err != nil {
			return toolErr(err)
		}
		out, err := buddy.SmartGroupFetchValue(cred, groupID)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})

	type groupIDIn struct {
		GroupID string `json:"group_id" jsonschema:"smart group id"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "smart_group_get_criteria",
		Description: "Return the criteria array for a Jamf Pro smart computer group. " +
			"Each criterion is a dict (name, priority, andOr, searchType, value, optional parens). " +
			"Smart groups use criteria for dynamic membership — use static_group_list_computers for explicit computer membership.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in groupIDIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		crit, err := buddy.SmartGroupCriteriaValue(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), strings.TrimSpace(in.GroupID))
		if err != nil {
			return toolErr(err)
		}
		n := len(crit)
		return toolOK(crit, &n)
	})

	type updateCriterionIn struct {
		GroupID       string `json:"group_id" jsonschema:"smart group id"`
		NameSubstring string `json:"name_substring" jsonschema:"case-insensitive substring matched against criterion name"`
		NewValue      string `json:"new_value" jsonschema:"new criterion value"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "smart_group_update_criterion_value",
		Description: "Update value on every smart-group criterion whose name contains name_substring (case-insensitive). " +
			"Common for bumping a version criterion. Mutating. Fails if no criteria match. " +
			"Prefer this over inventing a raw PUT to /api/v3/computer-groups/smart-groups.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in updateCriterionIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if err := buddy.SmartGroupUpdateCriterionValue(
			resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(),
			strings.TrimSpace(in.GroupID), in.NameSubstring, in.NewValue, logger,
		); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	type staticMembersIn struct {
		GroupID string `json:"group_id" jsonschema:"static computer group id"`
		Filter  string `json:"filter,omitempty" jsonschema:"client-side substring on id/name/serial"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_list_computers",
		Description: "List computers in a Jamf Pro static computer group (explicit membership). " +
			"Returns {ok, value: [{id,name,serial},...], count}. name may be empty when the API returns assignment IDs only. " +
			"Not for smart groups (use smart_group_get_criteria) and not Platform device_group_members.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticMembersIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		members, err := buddy.StaticGroupFetchMembers(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), strings.TrimSpace(in.GroupID))
		if err != nil {
			return toolErr(err)
		}
		members = buddy.FilterStaticGroupMembers(members, in.Filter)
		n := len(members)
		return toolOK(members, &n)
	})

	type staticMutateIn struct {
		GroupID     string `json:"group_id" jsonschema:"static computer group id"`
		ComputerIDs []any  `json:"computer_ids" jsonschema:"list of computer ids (int or string)"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	parseIDs := func(raw []any) ([]string, error) {
		out := make([]string, 0, len(raw))
		for i, v := range raw {
			switch t := v.(type) {
			case string:
				s := strings.TrimSpace(t)
				if s != "" {
					out = append(out, s)
				}
			case float64:
				out = append(out, strconv.FormatInt(int64(t), 10))
			case json.Number:
				out = append(out, t.String())
			case int:
				out = append(out, strconv.Itoa(t))
			case int64:
				out = append(out, strconv.FormatInt(t, 10))
			default:
				return nil, fmt.Errorf("computer_ids[%d]: want int or string, got %T", i, v)
			}
		}
		return out, nil
	}

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_add_computers",
		Description: "Add computer IDs to a Jamf Pro static computer group (merge; mutating). " +
			"No-op if all IDs are already members. Not for smart groups.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticMutateIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		ids, err := parseIDs(in.ComputerIDs)
		if err != nil {
			return toolErr(err)
		}
		if _, err := buddy.StaticGroupAddComputers(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), strings.TrimSpace(in.GroupID), ids, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_remove_computers",
		Description: "Remove computer IDs from a Jamf Pro static computer group (mutating). " +
			"No-op if none of the IDs are members. Not for smart groups.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticMutateIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		ids, err := parseIDs(in.ComputerIDs)
		if err != nil {
			return toolErr(err)
		}
		if _, err := buddy.StaticGroupRemoveComputers(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), strings.TrimSpace(in.GroupID), ids, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_set_computers",
		Description: "Replace the entire computer membership of a Jamf Pro static computer group (mutating). " +
			"Not for smart groups (criteria-based).",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticMutateIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		ids, err := parseIDs(in.ComputerIDs)
		if err != nil {
			return toolErr(err)
		}
		if _, err := buddy.StaticGroupSetComputers(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), strings.TrimSpace(in.GroupID), ids, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	type smartGroupCreateIn struct {
		Body map[string]any `json:"body" jsonschema:"smart group JSON (name required; criteria optional; id:-1 optional create convention)"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "smart_group_create",
		Description: "Create a Jamf Pro smart computer group (criteria-based; mutating). " +
			"body is full smart-group JSON (name required; criteria optional). Returns {ok, value: {id, href?}}. " +
			"Not for static groups — use static_group_create.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in smartGroupCreateIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if in.Body == nil {
			return toolErr(fmt.Errorf("body is required"))
		}
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		created, err := buddy.GroupCreateSmart(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), bodyJSON, logger)
		if err != nil {
			return toolErr(err)
		}
		if created == nil {
			return toolErr(fmt.Errorf("smart_group_create: cancelled"))
		}
		return toolOK(created, nil)
	})

	type staticGroupCreateIn struct {
		Body map[string]any `json:"body" jsonschema:"static group JSON (name required; assignments optional list of computer id strings)"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_create",
		Description: "Create a Jamf Pro static computer group (explicit membership; mutating). " +
			"body is static-group JSON (name required; assignments optional). Returns {ok, value: {id, href?}}. " +
			"Not for smart groups — use smart_group_create.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticGroupCreateIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if in.Body == nil {
			return toolErr(fmt.Errorf("body is required"))
		}
		bodyJSON, err := json.Marshal(in.Body)
		if err != nil {
			return toolErr(fmt.Errorf("encode body: %w", err))
		}
		created, err := buddy.GroupCreateStatic(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), bodyJSON, logger)
		if err != nil {
			return toolErr(err)
		}
		if created == nil {
			return toolErr(fmt.Errorf("static_group_create: cancelled"))
		}
		return toolOK(created, nil)
	})
}

func registerProfileTools(server *mcpsdk.Server, defaultAuth string, mutOpts func() buddy.Opts, logger *slog.Logger) {
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

	type profilesIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"name substring (case-insensitive)"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profiles",
		Description: "List Jamf macOS configuration profiles. Returns {ok, value: [{id,name},...], count}. " +
			"Optional filter matches name substring. Create with profile_create; clone with profile_clone; " +
			"edit with profile_xml → profile_xml_patch_payload → profile_put_xml (see docs mcp-tools / profile-payloads).",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profilesIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		recs, err := buddy.ProfileSearch(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), in.Filter, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(recs)
		return toolOK(recs, &n)
	})

	type profileIDIn struct {
		ProfileID int    `json:"profile_id" jsonschema:"configuration profile id"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_xml",
		Description: "Fetch full Classic XML for a macOS configuration profile by profile_id. " +
			"Returns {ok, value: xml string}. Prefer profile_xml_patch_payload for domain edits; " +
			"do not invent Classic PUT bodies from jamf_schema alone.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profileIDIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		xmlBytes, err := buddy.ProfileFetchXML(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), in.ProfileID)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(string(xmlBytes), nil)
	})

	type profileCreateIn struct {
		Name             string           `json:"name" jsonschema:"new profile name"`
		PayloadsPlistStr string           `json:"payloads_plist_str,omitempty" jsonschema:"payloads plist XML (or pass content)"`
		Content          []map[string]any `json:"content,omitempty" jsonschema:"PayloadContent array of payload dicts (alternative to payloads_plist_str)"`
		Category         string           `json:"category,omitempty" jsonschema:"Jamf category name"`
		ScopeAll         *bool            `json:"scope_all,omitempty" jsonschema:"scope all computers (default true when scope_group_ids empty)"`
		ScopeGroupIDs    []int            `json:"scope_group_ids,omitempty" jsonschema:"computer group ids to scope (implies not all computers)"`
		RedeployOnUpdate string           `json:"redeploy_on_update,omitempty" jsonschema:"Newly Assigned (default) or All"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_create",
		Description: "Create a Jamf macOS configuration profile (Classic POST; mutating). " +
			"Requires name plus payloads_plist_str or content (PayloadContent array of dicts). " +
			"Optional category, scope_all / scope_group_ids, redeploy_on_update. " +
			"Returns {ok, value: {id, name}}. Prefer payloads_set_content to build payloads_plist_str. " +
			"To copy an existing profile use profile_clone instead.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profileCreateIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		plistStr := strings.TrimSpace(in.PayloadsPlistStr)
		if len(in.Content) > 0 {
			items := make([]interface{}, 0, len(in.Content))
			for _, m := range in.Content {
				items = append(items, m)
			}
			updated, err := buddy.PlistSetPayloadContent(plistStr, items)
			if err != nil {
				return toolErr(err)
			}
			plistStr = updated
		}
		if plistStr == "" {
			return toolErr(fmt.Errorf("profile_create requires payloads_plist_str or content"))
		}
		scopeAll := len(in.ScopeGroupIDs) == 0
		if in.ScopeAll != nil {
			scopeAll = *in.ScopeAll
		}
		out, err := buddy.ProfileCreate(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), buddy.ProfileCreateSpec{
			Name:             in.Name,
			PayloadsPlist:    plistStr,
			Category:         in.Category,
			ScopeAll:         scopeAll,
			ScopeGroupIDs:    in.ScopeGroupIDs,
			RedeployOnUpdate: in.RedeployOnUpdate,
		}, logger)
		if err != nil {
			return toolErr(err)
		}
		if out == nil {
			return toolErr(fmt.Errorf("profile_create: cancelled"))
		}
		return toolOK(map[string]any{"id": out.ProfileID, "name": out.Name}, nil)
	})

	type profileCloneIn struct {
		ProfileID int    `json:"profile_id" jsonschema:"source configuration profile id"`
		Name      string `json:"name" jsonschema:"name for the new profile"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_clone",
		Description: "Clone a Jamf macOS configuration profile (mutating). " +
			"Copies payloads from profile_id into a new profile with name; scope is set to unassigned " +
			"(all_computers=false, no targets — not all computers; omitting scope in Classic can default to all). " +
			"Returns {ok}. Prefer profile_create for greenfield payloads.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profileCloneIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if err := buddy.ProfileClone(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), in.ProfileID, in.Name, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_redeploy",
		Description: "Redeploy a macOS configuration profile to all currently assigned devices (mutating). " +
			"Sets redeploy_on_update=All and PUTs without changing payloads. " +
			"Prefer profile_put_xml with redeploy=true when also writing content changes.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profileIDIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if err := buddy.ProfileRedeploy(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), in.ProfileID, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.ProfileID}, nil)
	})

	type profilePutIn struct {
		ProfileID int    `json:"profile_id" jsonschema:"configuration profile id"`
		XMLStr    string `json:"xml_str" jsonschema:"full profile XML"`
		Redeploy  bool   `json:"redeploy,omitempty" jsonschema:"if true, set redeploy_on_update=All before PUT"`
		Tenant  string `json:"tenant,omitempty" jsonschema:"tenant override for this call"`
		Surface string `json:"surface,omitempty" jsonschema:"instance or platform surface bias"`
		Auth    string `json:"auth,omitempty" jsonschema:"auth name pin (optional)"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_put_xml",
		Description: "PUT full Classic XML for a macOS configuration profile (mutating). " +
			"xml_str must be a complete os_x_configuration_profile document (e.g. from profile_xml after patch). " +
			"Optional redeploy=true sets redeploy_on_update=All. For create use profile_create; for redeploy-only use profile_redeploy.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profilePutIn) (*mcpsdk.CallToolResult, any, error) {
		endSess, sessErr := applySession(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth})
		if sessErr != nil {
			return toolErr(sessErr)
		}
		defer endSess()
		if err := buddy.ProfilePutXML(resolve(sessionIn{Tenant: in.Tenant, Surface: in.Surface, Auth: in.Auth}), mutOpts(), in.ProfileID, []byte(in.XMLStr), in.Redeploy, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.ProfileID}, nil)
	})

	type patchPayloadIn struct {
		ProfileXMLStr string `json:"profile_xml_str" jsonschema:"full profile XML"`
		Domain        string `json:"domain" jsonschema:"PayloadIdentifier (PayloadContent) or MCX preference-domain key"`
		NewPayload    any    `json:"new_payload" jsonschema:"PayloadContent: keys to merge (object or plist); MCX: settings dict only"`
		Tenant string `json:"tenant,omitempty" jsonschema:"unused; offline transform"`
		Surface string `json:"surface,omitempty" jsonschema:"unused; offline transform"`
		Auth string `json:"auth,omitempty" jsonschema:"unused; offline transform"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_xml_patch_payload",
		Description: "Offline: patch one payload domain in profile XML. " +
			"PayloadContent (domain is a PayloadIdentifier): new_payload keys are MERGED onto the existing payload " +
			"(omitted keys like PayloadUUID are preserved). " +
			"MCX (preference-domain key): new_payload is the settings dict under mcx_preference_settings only. " +
			"List PayloadContent domains first via profile_xml_extract_payloads + payloads_list_identifiers. " +
			"Returns {ok, value: updated profile XML}; then profile_put_xml. See docs profile-payloads.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in patchPayloadIn) (*mcpsdk.CallToolResult, any, error) {
		dictXML, err := payloadAnyToDictXML(in.NewPayload)
		if err != nil {
			return toolErr(err)
		}
		updated, err := buddy.PatchProfilePayload([]byte(in.ProfileXMLStr), strings.TrimSpace(in.Domain), dictXML)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(string(updated), nil)
	})

	type extractPayloadsIn struct {
		ProfileXMLStr string `json:"profile_xml_str" jsonschema:"full profile XML"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_xml_extract_payloads",
		Description: "Offline: extract the payloads plist from profile XML (<payloads> entity-decoded). " +
			"Returns {ok, value: payloads plist XML string}. Next: payloads_list_identifiers or starlark payloads_get_*.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in extractPayloadsIn) (*mcpsdk.CallToolResult, any, error) {
		out, err := buddy.ExtractPayloadsFromProfileXML([]byte(in.ProfileXMLStr))
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})

	type listIdentifiersIn struct {
		PayloadsPlistStr string `json:"payloads_plist_str" jsonschema:"payloads plist XML from profile_xml_extract_payloads"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "payloads_list_identifiers",
		Description: "Offline: list PayloadIdentifier strings under PayloadContent. " +
			"Returns {ok, value: [str,...], count}. Empty/nil when the plist is MCX-style (no PayloadContent array). " +
			"Not for MCX preference domains — use starlark payloads_get_mcx_dict after extract.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in listIdentifiersIn) (*mcpsdk.CallToolResult, any, error) {
		ids, err := buddy.PlistPayloadIdentifiers(in.PayloadsPlistStr)
		if err != nil {
			return toolErr(err)
		}
		if ids == nil {
			ids = []string{}
		}
		n := len(ids)
		return toolOK(ids, &n)
	})

	type payloadsSetContentIn struct {
		Content          []map[string]any `json:"content" jsonschema:"full PayloadContent array of payload dicts"`
		PayloadsPlistStr string           `json:"payloads_plist_str,omitempty" jsonschema:"existing payloads plist; omit or empty to start fresh"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "payloads_set_content",
		Description: "Offline: build a payloads plist with PayloadContent set to content. " +
			"Returns {ok, value: payloads plist XML}. Pass to profile_create as payloads_plist_str, " +
			"or wrap with starlark profile_xml_replace_payloads for existing profiles.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in payloadsSetContentIn) (*mcpsdk.CallToolResult, any, error) {
		if in.Content == nil {
			return toolErr(fmt.Errorf("content is required"))
		}
		items := make([]interface{}, 0, len(in.Content))
		for _, m := range in.Content {
			items = append(items, m)
		}
		out, err := buddy.PlistSetPayloadContent(in.PayloadsPlistStr, items)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(out, nil)
	})
}

func payloadAnyToDictXML(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", fmt.Errorf("new_payload is required")
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return "", fmt.Errorf("new_payload string is empty")
		}
		return buddy.NormaliseDictXML([]byte(s))
	case map[string]any:
		return buddy.EncodeDictMapToXML(t)
	default:
		// JSON numbers/etc. nested: re-marshal through map
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("new_payload: %w", err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return "", fmt.Errorf("new_payload must be object or string, got %T", v)
		}
		return buddy.EncodeDictMapToXML(m)
	}
}
