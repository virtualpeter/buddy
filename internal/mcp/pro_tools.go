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

func registerProComputerGroupTools(server *mcpsdk.Server, resolve func(credIn) string, mutOpts func() buddy.Opts, logger *slog.Logger) {
	type smartGroupIn struct {
		ID   string `json:"id,omitempty" jsonschema:"smart group id (pass id or name, not both)"`
		Name string `json:"name,omitempty" jsonschema:"smart group name (pass id or name, not both)"`
		Cred string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "smart_group",
		Description: "Load one Jamf Pro smart computer group (criteria-based membership) by id or name. " +
			"Returns {ok, value: full smart group JSON including criteria}. " +
			"Not a static group (explicit computer list) and not a Platform device_group. " +
			"Pass exactly one of id or name.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in smartGroupIn) (*mcpsdk.CallToolResult, any, error) {
		cred := resolve(credIn{Cred: in.Cred})
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
		Cred    string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "smart_group_get_criteria",
		Description: "Return the criteria array for a Jamf Pro smart computer group. " +
			"Each criterion is a dict (name, priority, andOr, searchType, value, optional parens). " +
			"Smart groups use criteria for dynamic membership — use static_group_list_computers for explicit computer membership.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in groupIDIn) (*mcpsdk.CallToolResult, any, error) {
		crit, err := buddy.SmartGroupCriteriaValue(resolve(credIn{Cred: in.Cred}), strings.TrimSpace(in.GroupID))
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
		Cred          string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "smart_group_update_criterion_value",
		Description: "Update value on every smart-group criterion whose name contains name_substring (case-insensitive). " +
			"Common for bumping a version criterion. Mutating. Fails if no criteria match. " +
			"Prefer this over inventing a raw PUT to /api/v3/computer-groups/smart-groups.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in updateCriterionIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.SmartGroupUpdateCriterionValue(
			resolve(credIn{Cred: in.Cred}), mutOpts(),
			strings.TrimSpace(in.GroupID), in.NameSubstring, in.NewValue, logger,
		); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	type staticMembersIn struct {
		GroupID string `json:"group_id" jsonschema:"static computer group id"`
		Filter  string `json:"filter,omitempty" jsonschema:"client-side substring on id/name/serial"`
		Cred    string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_list_computers",
		Description: "List computers in a Jamf Pro static computer group (explicit membership). " +
			"Returns {ok, value: [{id,name,serial},...], count}. name may be empty when the API returns assignment IDs only. " +
			"Not for smart groups (use smart_group_get_criteria) and not Platform device_group_members.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticMembersIn) (*mcpsdk.CallToolResult, any, error) {
		members, err := buddy.StaticGroupFetchMembers(resolve(credIn{Cred: in.Cred}), strings.TrimSpace(in.GroupID))
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
		Cred        string `json:"cred,omitempty" jsonschema:"credential name override"`
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
		ids, err := parseIDs(in.ComputerIDs)
		if err != nil {
			return toolErr(err)
		}
		if _, err := buddy.StaticGroupAddComputers(resolve(credIn{Cred: in.Cred}), mutOpts(), strings.TrimSpace(in.GroupID), ids, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_remove_computers",
		Description: "Remove computer IDs from a Jamf Pro static computer group (mutating). " +
			"No-op if none of the IDs are members. Not for smart groups.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticMutateIn) (*mcpsdk.CallToolResult, any, error) {
		ids, err := parseIDs(in.ComputerIDs)
		if err != nil {
			return toolErr(err)
		}
		if _, err := buddy.StaticGroupRemoveComputers(resolve(credIn{Cred: in.Cred}), mutOpts(), strings.TrimSpace(in.GroupID), ids, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "static_group_set_computers",
		Description: "Replace the entire computer membership of a Jamf Pro static computer group (mutating). " +
			"Not for smart groups (criteria-based).",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in staticMutateIn) (*mcpsdk.CallToolResult, any, error) {
		ids, err := parseIDs(in.ComputerIDs)
		if err != nil {
			return toolErr(err)
		}
		if _, err := buddy.StaticGroupSetComputers(resolve(credIn{Cred: in.Cred}), mutOpts(), strings.TrimSpace(in.GroupID), ids, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(nil, nil)
	})
}

func registerProfileTools(server *mcpsdk.Server, resolve func(credIn) string, mutOpts func() buddy.Opts, logger *slog.Logger) {
	type profilesIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"name substring (case-insensitive)"`
		Cred   string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profiles",
		Description: "List Jamf macOS configuration profiles. Returns {ok, value: [{id,name},...], count}. " +
			"Optional filter matches name substring. For payload edits use profile_xml → profile_xml_patch_payload → profile_put_xml " +
			"(see docs query \"mcp-tools\" or \"payload\").",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profilesIn) (*mcpsdk.CallToolResult, any, error) {
		recs, err := buddy.ProfileSearch(resolve(credIn{Cred: in.Cred}), in.Filter, logger)
		if err != nil {
			return toolErr(err)
		}
		n := len(recs)
		return toolOK(recs, &n)
	})

	type profileIDIn struct {
		ProfileID int    `json:"profile_id" jsonschema:"configuration profile id"`
		Cred      string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_xml",
		Description: "Fetch full Classic XML for a macOS configuration profile by profile_id. " +
			"Returns {ok, value: xml string}. Prefer profile_xml_patch_payload for domain edits; " +
			"do not invent Classic PUT bodies from jamf_schema alone.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profileIDIn) (*mcpsdk.CallToolResult, any, error) {
		xmlBytes, err := buddy.ProfileFetchXML(resolve(credIn{Cred: in.Cred}), in.ProfileID)
		if err != nil {
			return toolErr(err)
		}
		return toolOK(string(xmlBytes), nil)
	})

	type profilePutIn struct {
		ProfileID int    `json:"profile_id" jsonschema:"configuration profile id"`
		XMLStr    string `json:"xml_str" jsonschema:"full profile XML"`
		Redeploy  bool   `json:"redeploy,omitempty" jsonschema:"if true, set redeploy_on_update=All before PUT"`
		Cred      string `json:"cred,omitempty" jsonschema:"credential name override"`
	}
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "profile_put_xml",
		Description: "PUT full Classic XML for a macOS configuration profile (mutating). " +
			"xml_str must be a complete os_x_configuration_profile document (e.g. from profile_xml after patch). " +
			"Optional redeploy=true sets redeploy_on_update=All.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in profilePutIn) (*mcpsdk.CallToolResult, any, error) {
		if err := buddy.ProfilePutXML(resolve(credIn{Cred: in.Cred}), mutOpts(), in.ProfileID, []byte(in.XMLStr), in.Redeploy, logger); err != nil {
			return toolErr(err)
		}
		return toolOK(map[string]any{"id": in.ProfileID}, nil)
	})

	type patchPayloadIn struct {
		ProfileXMLStr string `json:"profile_xml_str" jsonschema:"full profile XML"`
		Domain        string `json:"domain" jsonschema:"PayloadIdentifier (PayloadContent) or MCX preference-domain key"`
		NewPayload    any    `json:"new_payload" jsonschema:"PayloadContent: keys to merge (object or plist); MCX: settings dict only"`
		Cred          string `json:"cred,omitempty" jsonschema:"unused; offline transform"`
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
