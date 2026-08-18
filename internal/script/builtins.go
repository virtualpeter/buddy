package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

// scriptOpts are the opts used for individual operations called from a script.
// SkipConfirm is always true so per-operation prompts are suppressed in scripts.
var scriptOpts = buddy.Opts{SkipConfirm: true, Verbose: false}

// runnerOpts reflects the top-level buddy invocation options (e.g. -y flag).
// Set by script.Run before executing a script.
var runnerOpts buddy.Opts

func resultOK(value starlark.Value) starlark.Value {
	if value == nil {
		return starlark.None
	}
	return value
}

// resultErr returns a Starlark-level failure (raises when returned as (nil, err)).
func resultErr(msg string) (starlark.Value, error) {
	return nil, fmt.Errorf("%s", msg)
}

// pageOptsFromArgs builds buddy.PageOpts from optional Starlark page / page_size.
// Omit page (None) → fetch all pages. page is 0-based when set.
func pageOptsFromArgs(page starlark.Value, pageSize starlark.Int) (buddy.PageOpts, error) {
	opts := buddy.PageOpts{Page: buddy.PageAll}
	if ps, ok := pageSize.Int64(); ok && ps > 0 {
		opts.PageSize = int(ps)
	}
	if page != nil && page != starlark.None {
		n, err := starlark.AsInt32(page)
		if err != nil {
			return opts, fmt.Errorf("page must be an int: %w", err)
		}
		if n < 0 {
			return opts, fmt.Errorf("page must be >= 0; omit page to fetch all")
		}
		opts.Page = n
	}
	return opts, nil
}

// formatTraceArgs renders positional and keyword arguments as a compact string.
func formatTraceArgs(args starlark.Tuple, kwargs []starlark.Tuple) string {
	var parts []string
	for _, a := range args {
		parts = append(parts, a.String())
	}
	for _, kv := range kwargs {
		parts = append(parts, fmt.Sprintf("%s=%s", kv[0].(starlark.String), kv[1].String()))
	}
	return strings.Join(parts, ", ")
}

type builtinFn func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)

func putBuiltin(dict starlark.StringDict, name string, fn builtinFn) {
	dict[name] = starlark.NewBuiltin(name, fn)
}

func makeBuiltins(credName string, logger *slog.Logger, trace bool) starlark.StringDict {
	dict := starlark.StringDict{}

	// Primary API (plural / singular).
	putBuiltin(dict, "policies", policiesBuiltin(credName, logger))
	putBuiltin(dict, "policy", policyBuiltin(credName, logger))
	putBuiltin(dict, "policy_xml", policyXMLBuiltin(credName, logger))
	putBuiltin(dict, "policy_set", policySetBuiltin(credName, logger))
	putBuiltin(dict, "packages", packagesBuiltin(credName, logger))
	putBuiltin(dict, "package", packageBuiltin(credName, logger))
	putBuiltin(dict, "scripts", scriptsBuiltin(credName, logger))
	putBuiltin(dict, "script", scriptBuiltin(credName, logger))
	putBuiltin(dict, "computers", computersBuiltin(credName, logger))
	putBuiltin(dict, "computer", computerBuiltin(credName, logger))
	putBuiltin(dict, "computer_detail", computerDetailBuiltin(credName, logger))
	putBuiltin(dict, "mdm_commands", mdmCommandsBuiltin(credName, logger))
	putBuiltin(dict, "profiles", profilesBuiltin(credName, logger))
	putBuiltin(dict, "profile", profileBuiltin(credName, logger))
	putBuiltin(dict, "profile_xml", profileXMLBuiltin(credName, logger))
	putBuiltin(dict, "smart_groups", smartGroupsBuiltin(credName, logger))
	putBuiltin(dict, "static_groups", staticGroupsBuiltin(credName, logger))
	putBuiltin(dict, "smart_group", smartGroupBuiltin(credName, logger))
	putBuiltin(dict, "static_group", staticGroupBuiltin(credName, logger))
	putBuiltin(dict, "roles", rolesBuiltin(credName, logger))
	putBuiltin(dict, "role", roleBuiltin(credName, logger))
	putBuiltin(dict, "role_privileges", rolePrivilegesBuiltin(credName, logger))
	putBuiltin(dict, "clients", clientsBuiltin(credName, logger))
	putBuiltin(dict, "client", clientBuiltin(credName, logger))
	registerMacAppBuiltins(dict, credName, logger)
	registerAppBuiltins(dict, credName, logger)
	putBuiltin(dict, "smart_group_get_version", smartGroupGetVersionNamed(credName, logger))
	putBuiltin(dict, "smart_group_set_version", smartGroupSetVersionNamed(credName, logger))
	putBuiltin(dict, "blueprints", blueprintsBuiltin(credName, logger))
	putBuiltin(dict, "blueprint", blueprintBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_create", blueprintCreateBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_set", blueprintSetBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_delete", blueprintDeleteBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_deploy", blueprintDeployBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_undeploy", blueprintUndeployBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_status", blueprintStatusBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_components", blueprintComponentsBuiltin(credName, logger))
	putBuiltin(dict, "blueprint_component", blueprintComponentBuiltin(credName, logger))
	registerPlatformBuiltins(dict, credName, logger)

	// Multi-step helpers / specialized actions.
	putBuiltin(dict, "policy_set_ss", policySetSS(credName, logger))
	putBuiltin(dict, "policy_set_install", policySetInstall(credName, logger))
	putBuiltin(dict, "policy_create_ss", policyCreateSS(credName, logger))
	putBuiltin(dict, "policy_create_package", policyCreatePackage(credName, logger))
	putBuiltin(dict, "policy_set_package", policySetPackage(credName, logger))
	putBuiltin(dict, "policy_set_scope_all", policySetScopeAll(credName, logger))
	putBuiltin(dict, "policy_delete", policyDelete(credName, logger))
	putBuiltin(dict, "policy_put_xml", policyPutXML(credName, logger))
	putBuiltin(dict, "policy_copy", policyCopy(credName, logger))
	putBuiltin(dict, "policy_migrate", policyMigrate(credName, logger))
	putBuiltin(dict, "smart_group_create", smartGroupCreate(credName, logger))
	registerCopyBuiltins(dict, credName, logger)
	putBuiltin(dict, "smart_group_put", smartGroupPut(credName, logger))
	putBuiltin(dict, "smart_group_get_criteria", smartGroupGetCriteria(credName, logger))
	putBuiltin(dict, "smart_group_set_criteria", smartGroupSetCriteria(credName, logger))
	putBuiltin(dict, "smart_group_find_criteria", smartGroupFindCriteria(credName, logger))
	putBuiltin(dict, "smart_group_update_criterion_value", smartGroupUpdateCriterionValue(credName, logger))
	putBuiltin(dict, "group_delete", groupDelete(credName, logger))
	putBuiltin(dict, "smart_group_list_computers", smartGroupListComputers(credName, logger))
	putBuiltin(dict, "static_group_list_computers", staticGroupListComputers(credName, logger))
	putBuiltin(dict, "static_group_create", staticGroupCreate(credName, logger))
	putBuiltin(dict, "static_group_add_computers", staticGroupAddComputers(credName, logger))
	putBuiltin(dict, "static_group_remove_computers", staticGroupRemoveComputers(credName, logger))
	putBuiltin(dict, "static_group_set_computers", staticGroupSetComputers(credName, logger))
	putBuiltin(dict, "role_put", rolePut(credName, logger))
	putBuiltin(dict, "role_create", roleCreate(credName, logger))
	putBuiltin(dict, "client_put", clientPut(credName, logger))
	putBuiltin(dict, "client_create", clientCreate(credName, logger))
	putBuiltin(dict, "error", errorBuiltin)
	putBuiltin(dict, "confirm", confirmBuiltin(logger))
	putBuiltin(dict, "auth", authBuiltin(credName, logger))
	putBuiltin(dict, "exit", exitBuiltin(logger))

	putBuiltin(dict, "jamf_request", jamfRequestBuiltin(credName, logger))
	putBuiltin(dict, "jamf_url", jamfURLBuiltin(credName, logger))
	putBuiltin(dict, "jamf_get", jamfGetBuiltin(credName, logger))
	putBuiltin(dict, "jamf_post", jamfPostBuiltin(credName, logger))
	putBuiltin(dict, "jamf_put", jamfPutBuiltin(credName, logger))
	putBuiltin(dict, "jamf_patch", jamfPatchBuiltin(credName, logger))
	putBuiltin(dict, "jamf_delete", jamfDeleteBuiltin(credName, logger))
	putBuiltin(dict, "jamf_schema", jamfSchemaBuiltin(credName, logger))

	putBuiltin(dict, "unwrap", unwrapBuiltin)
	putBuiltin(dict, "attempt", attemptBuiltin)
	putBuiltin(dict, "json_decode", jsonDecodeBuiltin)
	putBuiltin(dict, "json_encode", jsonEncodeBuiltin)
	putBuiltin(dict, "json_pretty", jsonPrettyBuiltin)
	putBuiltin(dict, "plist_decode", plistDecodeBuiltin)
	putBuiltin(dict, "plist_encode", plistEncodeBuiltin)
	putBuiltin(dict, "xml_escape", xmlEscapeBuiltin)
	putBuiltin(dict, "xml_unescape", xmlUnescapeBuiltin)
	putBuiltin(dict, "xml_sort", xmlSortBuiltin)

	putBuiltin(dict, "version_extract", versionExtractBuiltin)
	putBuiltin(dict, "version_compare", versionCompareBuiltin)
	putBuiltin(dict, "version_sort", versionSortBuiltin)

	putBuiltin(dict, "rsql_escape", rsqlEscapeBuiltin)
	putBuiltin(dict, "rsql_quote", rsqlQuoteBuiltin)
	putBuiltin(dict, "rsql_eq", rsqlEqBuiltin)
	putBuiltin(dict, "rsql_ne", rsqlNeBuiltin)
	putBuiltin(dict, "rsql_lt", rsqlLtBuiltin)
	putBuiltin(dict, "rsql_gt", rsqlGtBuiltin)
	putBuiltin(dict, "rsql_le", rsqlLeBuiltin)
	putBuiltin(dict, "rsql_ge", rsqlGeBuiltin)
	putBuiltin(dict, "rsql_contains", rsqlContainsBuiltin)
	putBuiltin(dict, "rsql_in", rsqlInBuiltin)
	putBuiltin(dict, "rsql_and", rsqlAndBuiltin)
	putBuiltin(dict, "rsql_or", rsqlOrBuiltin)
	putBuiltin(dict, "rsql_datetime", rsqlDatetimeBuiltin)
	putBuiltin(dict, "rsql_now", rsqlNowBuiltin)
	putBuiltin(dict, "rsql_days_ago", rsqlDaysAgoBuiltin)
	putBuiltin(dict, "rsql_hours_ago", rsqlHoursAgoBuiltin)

	putBuiltin(dict, "file_read", fileReadBuiltin)
	putBuiltin(dict, "file_read_bytes", fileReadBytesBuiltin)
	putBuiltin(dict, "file_write", fileWriteBuiltin)

	putBuiltin(dict, "step_set_package", stepSetPackageBuiltin)
	putBuiltin(dict, "step_clone_policy", stepClonePolicyBuiltin)
	putBuiltin(dict, "step_set_ss", stepSetSSBuiltin)
	putBuiltin(dict, "step_set_group_version", stepSetGroupVersionBuiltin)
	putBuiltin(dict, "step_set_install", stepSetInstallBuiltin)
	putBuiltin(dict, "step_enable_policy", stepEnablePolicyBuiltin)
	putBuiltin(dict, "step_disable_policy", stepDisablePolicyBuiltin)
	putBuiltin(dict, "step_set_trigger", stepSetTriggerBuiltin)
	putBuiltin(dict, "step_set_scope", stepSetScopeBuiltin)
	putBuiltin(dict, "step_set_scope_all", stepSetScopeAllBuiltin)
	putBuiltin(dict, "step_delete_policy", stepDeletePolicyBuiltin)
	putBuiltin(dict, "policy_template", policyTemplateBuiltin)
	putBuiltin(dict, "step_create_policy", stepCreatePolicyBuiltin)
	putBuiltin(dict, "step_note", stepNoteBuiltin)
	putBuiltin(dict, "step_with_note", stepWithNoteBuiltin)
	putBuiltin(dict, "change_batch", changeBatchBuiltin)
	putBuiltin(dict, "plan_change", planChangeBuiltin(credName, logger))
	putBuiltin(dict, "run_change", runChangeBuiltin(credName, logger))
	putBuiltin(dict, "run_workflow", runWorkflowBuiltin(credName, logger))

	putBuiltin(dict, "package_upload", packageUpload(credName, logger))
	putBuiltin(dict, "packages_scan", packagesScanBuiltin(credName, logger))
	putBuiltin(dict, "package_ready", packageReady(credName, logger))
	putBuiltin(dict, "package_wait_ready", packageWaitReady(credName, logger))
	putBuiltin(dict, "package_download", packageDownload(credName, logger))
	putBuiltin(dict, "package_delete", packageDelete(credName, logger))
	putBuiltin(dict, "package_put", packagePut(credName, logger))

	putBuiltin(dict, "uuid_new", uuidNewBuiltin)
	putBuiltin(dict, "uuid_is", uuidIsBuiltin)
	putBuiltin(dict, "profile_create", profileCreate(credName, logger))
	putBuiltin(dict, "profile_clone", profileClone(credName, logger))
	putBuiltin(dict, "profile_copy", profileCopy(credName, logger))
	putBuiltin(dict, "profile_migrate", profileMigrate(credName, logger))
	putBuiltin(dict, "profile_create_xml", profileCreateXML(credName, logger))
	putBuiltin(dict, "profile_delete", profileDelete(credName, logger))
	putBuiltin(dict, "profile_redeploy", profileRedeploy(credName, logger))
	putBuiltin(dict, "profile_acs_bind_uuid", profileACSBindUUIDBuiltin)
	putBuiltin(dict, "profile_get_payloads", profileGetPayloads(credName, logger))
	putBuiltin(dict, "profile_get_payload", profileGetPayload(credName, logger))
	putBuiltin(dict, "profile_list_payloads", profileListPayloads(credName, logger))
	putBuiltin(dict, "profile_set_payload", profileSetPayload(credName, logger))
	putBuiltin(dict, "profile_set_xml", profileSetXML(logger))
	putBuiltin(dict, "profile_put_xml", profilePutXML(credName, logger))
	putBuiltin(dict, "profile_xml_extract_payloads", profileXMLExtractPayloadsBuiltin)
	putBuiltin(dict, "profile_xml_randomize_uuids", profileXMLRandomizeUUIDsBuiltin)
	putBuiltin(dict, "profile_xml_prepare_for_create", profileXMLPrepareForCreateBuiltin)
	putBuiltin(dict, "profile_xml_replace_payloads", profileXMLReplacePayloadsBuiltin)
	putBuiltin(dict, "profile_xml_set_redeploy_all", profileXMLSetRedeployAllBuiltin)
	putBuiltin(dict, "profile_xml_patch_payload", profileXMLPatchPayloadBuiltin)
	putBuiltin(dict, "payloads_list_identifiers", payloadsListIdentifiersBuiltin)
	putBuiltin(dict, "payloads_collect_uuids", payloadsCollectUUIDsBuiltin)
	putBuiltin(dict, "payloads_get_payload", payloadsGetPayloadBuiltin)
	putBuiltin(dict, "payloads_replace_payload", payloadsReplacePayload(logger))
	putBuiltin(dict, "payloads_replace_mcx_dict", payloadsReplaceMCXDictBuiltin)
	putBuiltin(dict, "payloads_get_mcx_dict", payloadsGetMCXDictBuiltin)
	putBuiltin(dict, "payloads_set_content", payloadsSetContentBuiltin)
	putBuiltin(dict, "payloads_add_payload", payloadsAddPayloadBuiltin)
	putBuiltin(dict, "payloads_remove_payload", payloadsRemovePayloadBuiltin)
	putBuiltin(dict, "plist_normalise_dict", plistNormaliseDictBuiltin)
	putBuiltin(dict, "pppc_fda", pppcFDABuiltin)
	putBuiltin(dict, "pppc_payload", pppcPayloadBuiltin)
	putBuiltin(dict, "prefs_payload", prefsPayloadBuiltin)
	putBuiltin(dict, "step_set_profile_payload", stepSetProfilePayloadBuiltin)
	putBuiltin(dict, "step_set_profile_xml", stepSetProfileXMLBuiltin)
	putBuiltin(dict, "step_create_package_policy", stepCreatePackagePolicyBuiltin)
	putBuiltin(dict, "step_create_profile", stepCreateProfileBuiltin)
	putBuiltin(dict, "step_create_ss", stepCreateSSBuiltin)

	putBuiltin(dict, "script_get_contents", scriptGetContents(credName, logger))
	putBuiltin(dict, "script_create", scriptCreate(credName, logger))
	putBuiltin(dict, "script_update", scriptUpdate(credName, logger))
	putBuiltin(dict, "script_put", scriptPutBuiltin(credName, logger))
	putBuiltin(dict, "script_delete", scriptDelete(credName, logger))

	putBuiltin(dict, "computer_send_mdm_command", computerSendMDMCommand(credName, logger))
	putBuiltin(dict, "computer_reboot", computerReboot(credName, logger))
	putBuiltin(dict, "computer_redeploy", computerRedeploy(credName, logger))
	putBuiltin(dict, "computer_shutdown", computerShutdown(credName, logger))
	putBuiltin(dict, "computer_lock", computerLock(credName, logger))
	putBuiltin(dict, "computer_logout", computerLogout(credName, logger))
	putBuiltin(dict, "computer_erase", computerErase(credName, logger))
	putBuiltin(dict, "computer_device_info", computerDeviceInfo(credName, logger))
	putBuiltin(dict, "computer_device_location", computerDeviceLocation(credName, logger))
	putBuiltin(dict, "computer_security_info", computerSecurityInfo(credName, logger))
	putBuiltin(dict, "computer_installed_apps", computerInstalledApps(credName, logger))
	putBuiltin(dict, "computer_managed_apps", computerManagedApps(credName, logger))
	putBuiltin(dict, "computer_certificates", computerCertificates(credName, logger))

	if trace {
		for name, val := range dict {
			b, ok := val.(*starlark.Builtin)
			if !ok {
				continue
			}
			bName := name
			orig := b
			dict[bName] = starlark.NewBuiltin(bName, func(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
				pos := t.CallFrame(0).Pos
				w := writerFromLocal(t, localStderr, os.Stderr)
				fmt.Fprintf(w, "[trace] %s:%d  → %s(%s)\n", pos.Filename(), pos.Line, bName, formatTraceArgs(args, kwargs))
				result, err := starlark.Call(t, orig, args, kwargs)
				if err != nil {
					fmt.Fprintf(w, "[trace] %s:%d  ← %s error: %v\n", pos.Filename(), pos.Line, bName, err)
				} else {
					fmt.Fprintf(w, "[trace] %s:%d  ← %s = %s\n", pos.Filename(), pos.Line, bName, result)
				}
				return result, err
			})
		}
	}
	wrapDeniedBuiltins(dict)
	return dict
}

// wrapDeniedBuiltins replaces channel-denied builtins with raisers so calls fail loudly.
func wrapDeniedBuiltins(dict starlark.StringDict) {
	patterns := buddy.ActiveChannelDenyBuiltins()
	if len(patterns) == 0 {
		return
	}
	channel := buddy.ActiveChannel()
	for name, val := range dict {
		if _, ok := val.(*starlark.Builtin); !ok {
			continue
		}
		if !buddy.BuiltinNameDenied(patterns, name) {
			continue
		}
		bName := name
		dict[bName] = starlark.NewBuiltin(bName, func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			return nil, buddy.ErrBuiltinDenied(channel, bName)
		})
	}
}

func policyDetailsToStarlark(d *buddy.PolicyDetails) *starlark.Dict {
	if d == nil {
		return new(starlark.Dict)
	}
	out := new(starlark.Dict)
	_ = out.SetKey(starlark.String("id"), starlark.MakeInt(d.ID))
	_ = out.SetKey(starlark.String("name"), starlark.String(d.Name))
	_ = out.SetKey(starlark.String("enabled"), starlark.Bool(d.Enabled))
	_ = out.SetKey(starlark.String("trigger"), starlark.String(d.Trigger))
	_ = out.SetKey(starlark.String("trigger_other"), starlark.String(d.TriggerOther))
	_ = out.SetKey(starlark.String("frequency"), starlark.String(d.Frequency))
	scopeDict := new(starlark.Dict)
	_ = scopeDict.SetKey(starlark.String("all_computers"), starlark.Bool(d.Scope.AllComputers))
	_ = scopeDict.SetKey(starlark.String("all_jss_users"), starlark.Bool(d.Scope.AllJSSUsers))
	scopeGroupList := starlark.NewList(nil)
	for _, gid := range d.Scope.ComputerGroupIDs {
		_ = scopeGroupList.Append(starlark.MakeInt(gid))
	}
	_ = scopeDict.SetKey(starlark.String("computer_group_ids"), scopeGroupList)
	_ = out.SetKey(starlark.String("scope"), scopeDict)
	pkgList := starlark.NewList(nil)
	for _, p := range d.Packages {
		pd := new(starlark.Dict)
		_ = pd.SetKey(starlark.String("id"), starlark.MakeInt(p.ID))
		_ = pd.SetKey(starlark.String("name"), starlark.String(p.Name))
		_ = pd.SetKey(starlark.String("action"), starlark.String(p.Action))
		_ = pkgList.Append(pd)
	}
	_ = out.SetKey(starlark.String("packages"), pkgList)
	scriptList := starlark.NewList(nil)
	for _, s := range d.Scripts {
		sd := new(starlark.Dict)
		_ = sd.SetKey(starlark.String("id"), starlark.String(s.ID))
		_ = sd.SetKey(starlark.String("name"), starlark.String(s.Name))
		_ = sd.SetKey(starlark.String("priority"), starlark.String(s.Priority))
		paramsDict := new(starlark.Dict)
		for k, v := range s.Params {
			_ = paramsDict.SetKey(starlark.String(k), starlark.String(v))
		}
		_ = sd.SetKey(starlark.String("params"), paramsDict)
		_ = scriptList.Append(sd)
	}
	_ = out.SetKey(starlark.String("scripts"), scriptList)
	return out
}

func policySetSS(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var version starlark.String
		var groupIDs *starlark.List
		var displayName, description, installText, reinstallText starlark.String
		var forceView, feature, selfService *starlark.Bool
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_set_ss", args, kwargs,
			"id", &policyID,
			"version?", &version, "group_ids?", &groupIDs,
			"display_name?", &displayName, "description?", &description,
			"install_text?", &installText, "reinstall_text?", &reinstallText,
			"force_view?", &forceView, "feature?", &feature, "self_service?", &selfService,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		pid, _ := policyID.Int64()
		ssOpts := buddy.PolicySSOpts{Version: string(version)}
		if groupIDs != nil && groupIDs.Len() > 0 {
			for i := 0; i < groupIDs.Len(); i++ {
				v := groupIDs.Index(i)
				if n, ok := v.(starlark.Int); ok {
					n64, _ := n.Int64()
					ssOpts.GroupIDs = append(ssOpts.GroupIDs, int(n64))
				}
			}
		}
		if displayName != "" {
			s := string(displayName)
			ssOpts.DisplayName = &s
		}
		if description != "" {
			s := string(description)
			ssOpts.Description = &s
		}
		if installText != "" {
			s := string(installText)
			ssOpts.InstallText = &s
		}
		if reinstallText != "" {
			s := string(reinstallText)
			ssOpts.ReinstallText = &s
		}
		if forceView != nil {
			b := bool(*forceView)
			ssOpts.ForceView = &b
		}
		if feature != nil {
			b := bool(*feature)
			ssOpts.Feature = &b
		}
		if selfService != nil {
			b := bool(*selfService)
			ssOpts.SelfService = &b
		}
		err := buddy.PolicySSUpdate(c, scriptOpts, int(pid), ssOpts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func policySetInstall(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var version starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_set_install", args, kwargs, "id", &policyID, "version", &version,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		pid, _ := policyID.Int64()
		paramIndices := []int{4, 5, 6}
		err := buddy.PolicyInstall(c, scriptOpts, int(pid), string(version), paramIndices, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func disableModeFromKeepSource(keepSource starlark.Value) (buddy.DisableSourceMode, error) {
	if keepSource == nil || keepSource == starlark.None {
		return buddy.DisableSourceAuto, nil
	}
	b, ok := keepSource.(starlark.Bool)
	if !ok {
		return buddy.DisableSourceAuto, fmt.Errorf("keep_source must be a bool")
	}
	if bool(b) {
		return buddy.DisableSourceKeep, nil
	}
	return buddy.DisableSourceForce, nil
}

func disableModeFromPackageKwargs(disableSource, keepSource starlark.Value) (buddy.DisableSourceMode, error) {
	hasDisable := disableSource != nil && disableSource != starlark.None
	hasKeep := keepSource != nil && keepSource != starlark.None
	if hasDisable && hasKeep {
		return buddy.DisableSourceAuto, fmt.Errorf("disable_source and keep_source are mutually exclusive")
	}
	if hasKeep {
		b, ok := keepSource.(starlark.Bool)
		if !ok {
			return buddy.DisableSourceAuto, fmt.Errorf("keep_source must be a bool")
		}
		if bool(b) {
			return buddy.DisableSourceKeep, nil
		}
		return buddy.DisableSourceForce, nil
	}
	if hasDisable {
		b, ok := disableSource.(starlark.Bool)
		if !ok {
			return buddy.DisableSourceAuto, fmt.Errorf("disable_source must be a bool")
		}
		if bool(b) {
			return buddy.DisableSourceForce, nil
		}
		return buddy.DisableSourceKeep, nil
	}
	return buddy.DisableSourceAuto, nil
}

func policyCreateSS(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var trigger, displayName, category starlark.String
		var scopeGroupID starlark.Int = starlark.MakeInt(0)
		var scriptID starlark.Int = starlark.MakeInt(0)
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_create_ss", args, kwargs,
			"trigger", &trigger, "display_name", &displayName,
			"category?", &category,
			"scope_group_id?", &scopeGroupID,
			"script_id?", &scriptID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		sgid, _ := scopeGroupID.Int64()
		sid, _ := scriptID.Int64()
		policyID, err := buddy.PolicyCreateSS(c, scriptOpts, string(trigger), string(displayName), string(category), int(sgid), int(sid), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.MakeInt(policyID)), nil
	}
}

func policyCreatePackage(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, version, action, category, packagePattern starlark.String
		var scopeAll starlark.Value = starlark.None
		var scopeGroupIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_create_package", args, kwargs,
			"name", &name, "version", &version,
			"action?", &action,
			"scope_all?", &scopeAll,
			"scope_group_ids?", &scopeGroupIDs,
			"category?", &category,
			"package_pattern?", &packagePattern,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		act := string(action)
		if act == "" {
			act = "install"
		}
		var groupIDs []int
		if scopeGroupIDs != nil {
			for i := 0; i < scopeGroupIDs.Len(); i++ {
				n, ok := scopeGroupIDs.Index(i).(starlark.Int)
				if !ok {
					return resultErr("scope_group_ids must be a list of ints")
				}
				v64, _ := n.Int64()
				groupIDs = append(groupIDs, int(v64))
			}
		}
		all := len(groupIDs) == 0
		if scopeAll != nil && scopeAll != starlark.None {
			b, ok := scopeAll.(starlark.Bool)
			if !ok {
				return resultErr("scope_all must be a bool")
			}
			all = bool(b)
		}
		out, err := buddy.PolicyCreatePackage(c, scriptOpts, string(name), string(version), act, all, groupIDs, string(category), string(packagePattern), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("policy_create_package: cancelled")
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.MakeInt(out.PolicyID))
		_ = d.SetKey(starlark.String("name"), starlark.String(out.PolicyName))
		_ = d.SetKey(starlark.String("trigger"), starlark.String(out.Trigger))
		return resultOK(d), nil
	}
}

func policySetPackage(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, version, trigger, packagePattern starlark.String
		var policyID starlark.Int = starlark.MakeInt(0)
		disableSource := starlark.Value(starlark.None)
		keepSource := starlark.Value(starlark.None)
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_set_package", args, kwargs,
			"name", &name, "version", &version,
			"id?", &policyID, "trigger?", &trigger, "package_pattern?", &packagePattern,
			"disable_source?", &disableSource, "keep_source?", &keepSource,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		pid, _ := policyID.Int64()
		triggerOverride := ""
		if trigger != "" {
			triggerOverride = string(trigger)
		}
		packageOverride := ""
		if packagePattern != "" {
			packageOverride = string(packagePattern)
		}
		disableMode, err := disableModeFromPackageKwargs(disableSource, keepSource)
		if err != nil {
			return nil, err
		}
		err = buddy.PolicyPackage(c, scriptOpts, strings.ToLower(string(name)), string(version), int(pid), triggerOverride, packageOverride, disableMode, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// group_list_smart(filter?, page?, page_size?, tenant?, surface?, auth?) → {ok, value: [{id, name, description}, ...]}
func groupListSmart(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return groupListBuiltin("group_list_smart", buddy.GroupSearchSmart, credName, logger)
}

// group_list_static(filter?, page?, page_size?, tenant?, surface?, auth?) → {ok, value: [{id, name, description}, ...]}
func groupListStatic(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return groupListBuiltin("group_list_static", buddy.GroupSearchStatic, credName, logger)
}

type groupSearchFunc func(credName, filter string, opts buddy.PageOpts, logger *slog.Logger) ([]buddy.GroupSummary, error)

func groupListBuiltin(name string, search groupSearchFunc, credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs(name, args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		records, err := search(c, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, g := range records {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(g.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(g.Name))
			_ = d.SetKey(starlark.String("description"), starlark.String(g.Description))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// smart_group_list_computers(group_id, filter?, tenant?, surface?, auth?) → {ok, value: list[dict]}
func smartGroupListComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_list_computers", args, kwargs, "id", &groupID, "filter?", &filter,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		members, err := buddy.SmartGroupFetchMembers(c, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		members = buddy.FilterStaticGroupMembers(members, string(filter))
		list := starlark.NewList(nil)
		for _, m := range members {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(m.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(m.Name))
			_ = d.SetKey(starlark.String("serial"), starlark.String(m.Serial))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// static_group_list_computers(group_id, filter?, tenant?, surface?, auth?) → {ok, value: list[dict]}
func staticGroupListComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("static_group_list_computers", args, kwargs, "id", &groupID, "filter?", &filter,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		members, err := buddy.StaticGroupFetchMembers(c, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		members = buddy.FilterStaticGroupMembers(members, string(filter))
		list := starlark.NewList(nil)
		for _, m := range members {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(m.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(m.Name))
			_ = d.SetKey(starlark.String("serial"), starlark.String(m.Serial))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

func parseComputerIDStringsFromList(list *starlark.List, fn string) ([]string, error) {
	var out []string
	for i := 0; i < list.Len(); i++ {
		v := list.Index(i)
		switch t := v.(type) {
		case starlark.Int:
			n64, _ := t.Int64()
			out = append(out, strconv.FormatInt(n64, 10))
		case starlark.String:
			s := strings.TrimSpace(string(t))
			if s != "" {
				out = append(out, s)
			}
		default:
			return nil, fmt.Errorf("%s: computer_ids entries must be int or string", fn)
		}
	}
	return out, nil
}

// static_group_add_computers(group_id, computer_ids, tenant?, surface?, auth?) → {ok, value: None}
func staticGroupAddComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("static_group_add_computers", args, kwargs, "id", &groupID, "computer_ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, err := parseComputerIDStringsFromList(computerIDs, "static_group_add_computers")
		if err != nil {
			return nil, err
		}
		_, err = buddy.StaticGroupAddComputers(c, scriptOpts, string(groupID), ids, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// static_group_remove_computers(group_id, computer_ids, tenant?, surface?, auth?) → {ok, value: None}
func staticGroupRemoveComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("static_group_remove_computers", args, kwargs, "id", &groupID, "computer_ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, err := parseComputerIDStringsFromList(computerIDs, "static_group_remove_computers")
		if err != nil {
			return nil, err
		}
		_, err = buddy.StaticGroupRemoveComputers(c, scriptOpts, string(groupID), ids, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// static_group_set_computers(group_id, computer_ids, tenant?, surface?, auth?) → {ok, value: None}
func staticGroupSetComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("static_group_set_computers", args, kwargs, "id", &groupID, "computer_ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, err := parseComputerIDStringsFromList(computerIDs, "static_group_set_computers")
		if err != nil {
			return nil, err
		}
		_, err = buddy.StaticGroupSetComputers(c, scriptOpts, string(groupID), ids, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// smart_group_create(body, tenant?, surface?, auth?) → {ok, value: dict {"id", "href"?}}
func smartGroupCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_create", args, kwargs, "body", &body,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err))
		}
		opts := scriptOpts
		opts.SkipConfirm = true
		created, err := buddy.GroupCreateSmart(c, opts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if created == nil {
			return resultErr("smart_group_create: cancelled")
		}
		return groupCreateResultValue(created)
	}
}

// static_group_create(body, tenant?, surface?, auth?) → {ok, value: dict {"id", "href"?}}
func staticGroupCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("static_group_create", args, kwargs, "body", &body,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err))
		}
		opts := scriptOpts
		opts.SkipConfirm = true
		created, err := buddy.GroupCreateStatic(c, opts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if created == nil {
			return resultErr("static_group_create: cancelled")
		}
		return groupCreateResultValue(created)
	}
}

func groupCreateResultValue(created *buddy.GroupCreateResult) (starlark.Value, error) {
	b, err := json.Marshal(created)
	if err != nil {
		return resultErr(err.Error())
	}
	val, err := JSONDecodeToStarlark(string(b))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(val), nil
}

// smart_group_put(group_id, body, tenant?, surface?, auth?) → {ok, value: None}
func smartGroupPut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_put", args, kwargs, "id", &groupID, "body", &body,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err))
		}
		// For scripts, always skip confirm.
		opts := scriptOpts
		opts.SkipConfirm = true
		if err := buddy.GroupPutSmartRaw(c, opts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// smart_group_get_criteria(group_id, tenant?, surface?, auth?) → {ok, value: list[dict]}
func smartGroupGetCriteria(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_get_criteria", args, kwargs, "id", &groupID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		body, err := buddy.GroupFetchSmartRaw(c, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err))
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict")
		}
		critVal, found, err := d.Get(starlark.String("criteria"))
		if err != nil {
			return resultErr(fmt.Sprintf("lookup criteria: %v", err))
		}
		if !found {
			// No criteria field: return empty list.
			return resultOK(starlark.NewList(nil)), nil
		}
		if _, ok := critVal.(*starlark.List); !ok {
			return resultErr("smart group criteria is not a list")
		}
		return resultOK(critVal), nil
	}
}

// smart_group_set_criteria(group_id, criteria, tenant?, surface?, auth?) → {ok, value: None}
func smartGroupSetCriteria(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var criteria *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_set_criteria", args, kwargs, "id", &groupID, "criteria", &criteria,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		body, err := buddy.GroupFetchSmartRaw(c, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err))
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict")
		}
		if err := d.SetKey(starlark.String("criteria"), criteria); err != nil {
			return resultErr(fmt.Sprintf("set criteria: %v", err))
		}
		jsonStr, err := StarlarkToJSON(d)
		if err != nil {
			return resultErr(fmt.Sprintf("encode updated smart group JSON: %v", err))
		}
		opts := scriptOpts
		opts.SkipConfirm = true
		if err := buddy.GroupPutSmartRaw(c, opts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// smart_group_find_criteria(group_id, name_substring, tenant?, surface?, auth?) → {ok, value: list[dict]}
func smartGroupFindCriteria(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, nameSubstr starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_find_criteria", args, kwargs, "id", &groupID, "name_substring", &nameSubstr,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		body, err := buddy.GroupFetchSmartRaw(c, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err))
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict")
		}
		critVal, found, err := d.Get(starlark.String("criteria"))
		if err != nil {
			return resultErr(fmt.Sprintf("lookup criteria: %v", err))
		}
		if !found {
			return resultOK(starlark.NewList(nil)), nil
		}
		lst, ok := critVal.(*starlark.List)
		if !ok {
			return resultErr("smart group criteria is not a list")
		}
		nameLower := strings.ToLower(string(nameSubstr))
		out := starlark.NewList(nil)
		for i := 0; i < lst.Len(); i++ {
			item := lst.Index(i)
			cd, ok := item.(*starlark.Dict)
			if !ok {
				continue
			}
			nameVal, found, _ := cd.Get(starlark.String("name"))
			if !found {
				continue
			}
			nameStr, ok := starlark.AsString(nameVal)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(nameStr), nameLower) {
				_ = out.Append(cd)
			}
		}
		return resultOK(out), nil
	}
}

// smart_group_update_criterion_value(group_id, name_substring, new_value, tenant?, surface?, auth?) → {ok, value: None}
func smartGroupUpdateCriterionValue(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, nameSubstr, newValue starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_update_criterion_value", args, kwargs,
			"id", &groupID, "name_substring", &nameSubstr, "new_value", &newValue,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		body, err := buddy.GroupFetchSmartRaw(c, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err))
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict")
		}
		critVal, found, err := d.Get(starlark.String("criteria"))
		if err != nil {
			return resultErr(fmt.Sprintf("lookup criteria: %v", err))
		}
		if !found {
			return resultErr("smart group has no criteria array")
		}
		lst, ok := critVal.(*starlark.List)
		if !ok {
			return resultErr("smart group criteria is not a list")
		}
		nameLower := strings.ToLower(string(nameSubstr))
		changed := false
		for i := 0; i < lst.Len(); i++ {
			item := lst.Index(i)
			cd, ok := item.(*starlark.Dict)
			if !ok {
				continue
			}
			nameVal, found, _ := cd.Get(starlark.String("name"))
			if !found {
				continue
			}
			nameStr, ok := starlark.AsString(nameVal)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(nameStr), nameLower) {
				if err := cd.SetKey(starlark.String("value"), newValue); err != nil {
					return resultErr(fmt.Sprintf("set criterion value: %v", err))
				}
				changed = true
			}
		}
		if !changed {
			return resultErr(fmt.Sprintf("no criteria found with name containing %q", string(nameSubstr)))
		}
		jsonStr, err := StarlarkToJSON(d)
		if err != nil {
			return resultErr(fmt.Sprintf("encode updated smart group JSON: %v", err))
		}
		opts := scriptOpts
		opts.SkipConfirm = true
		if err := buddy.GroupPutSmartRaw(c, opts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func rolePrivilegesList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var sess sessionArgs
		if err := starlark.UnpackArgs("role_privileges_list", args, kwargs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		privs, err := buddy.RolePrivilegesList(c, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		elems := make([]starlark.Value, len(privs))
		for i, p := range privs {
			elems[i] = starlark.String(p)
		}
		return resultOK(starlark.NewList(elems)), nil
	}
}

// role_put(role_id, dict, tenant?, surface?, auth?) → {ok, value: dict}
func rolePut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var roleID starlark.String
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("role_put", args, kwargs, "id", &roleID, "dict", &bodyVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error())
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict")
		}
		// Resolve desired fields, preserving existing where unspecified.
		existing, err := buddy.RoleFetch(c, string(roleID), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		displayName := existing.DisplayName
		if v, ok := m["displayName"].(string); ok && v != "" {
			displayName = v
		}
		privs := existing.Privileges
		if v, ok := m["privileges"]; ok {
			switch arr := v.(type) {
			case []interface{}:
				var out []string
				for _, item := range arr {
					if s, ok := item.(string); ok {
						out = append(out, s)
					}
				}
				if out != nil {
					privs = out
				}
			case []string:
				privs = arr
			}
		}
		updated, err := buddy.RoleUpdateWithResult(c, string(roleID), scriptOpts, displayName, privs, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if updated == nil {
			return resultErr("role_put: cancelled")
		}
		b, err := json.Marshal(updated)
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func roleCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Backward compatible signature:
		//   role_create(display_name, privileges, tenant?, surface?, auth?)
		// Component signature:
		//   role_create(dict, tenant?, surface?, auth?)
		if len(args) > 0 {
			if d, ok := args[0].(*starlark.Dict); ok {
				var sess sessionArgs
				_ = starlark.UnpackArgs("role_create", starlark.Tuple{}, kwargs,
					"tenant?", &sess.Tenant,
					"surface?", &sess.Surface,
					"auth?", &sess.Auth)
				sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
				if sessErr != nil {
					return resultErr(sessErr.Error())
				}
				defer restoreSession()
				c := sessionPin
				g, err := StarlarkValueToGo(d)
				if err != nil {
					return resultErr(err.Error())
				}
				m, ok := g.(map[string]interface{})
				if !ok {
					return resultErr("role_create: dict must be a map")
				}
				displayName, _ := m["displayName"].(string)
				var privs []string
				if v, ok := m["privileges"]; ok {
					switch arr := v.(type) {
					case []interface{}:
						for _, item := range arr {
							if s, ok := item.(string); ok {
								privs = append(privs, s)
							}
						}
					case []string:
						privs = arr
					}
				}
				if displayName == "" {
					return resultErr("role_create: missing displayName")
				}
				role, err := buddy.RoleCreateWithID(c, scriptOpts, displayName, privs, logger)
				if err != nil {
					return resultErr(err.Error())
				}
				if role == nil {
					return resultErr("role_create: cancelled")
				}
				return resultOK(starlark.String(role.ID)), nil
			}
		}

		var displayName starlark.String
		var privileges *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("role_create", args, kwargs, "display_name", &displayName, "privileges", &privileges,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		var privs []string
		for i := 0; i < privileges.Len(); i++ {
			v := privileges.Index(i)
			if s, ok := v.(starlark.String); ok {
				privs = append(privs, string(s))
			} else {
				return nil, fmt.Errorf("role_create: privileges must be a list of strings")
			}
		}
		role, err := buddy.RoleCreateWithID(c, scriptOpts, string(displayName), privs, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if role == nil {
			return resultErr("role_create: cancelled")
		}
		return resultOK(starlark.String(role.ID)), nil
	}
}

// client_put(client_id, dict, tenant?, surface?, auth?) → {ok, value: dict}
func clientPut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var clientID starlark.String
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("client_put", args, kwargs, "id", &clientID, "dict", &bodyVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error())
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict")
		}
		updated, err := buddy.ClientPutFields(c, scriptOpts, string(clientID), m, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if updated == nil {
			return resultErr("client_put: cancelled")
		}
		b, err := json.Marshal(updated)
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func clientCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Backward compatible signature:
		//   client_create(display_name, scopes, tenant?, surface?, auth?)
		// Component signature:
		//   client_create(dict, tenant?, surface?, auth?)
		if len(args) > 0 {
			if d, ok := args[0].(*starlark.Dict); ok {
				var sess sessionArgs
				_ = starlark.UnpackArgs("client_create", starlark.Tuple{}, kwargs,
					"tenant?", &sess.Tenant,
					"surface?", &sess.Surface,
					"auth?", &sess.Auth)
				sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
				if sessErr != nil {
					return resultErr(sessErr.Error())
				}
				defer restoreSession()
				c := sessionPin
				g, err := StarlarkValueToGo(d)
				if err != nil {
					return resultErr(err.Error())
				}
				m, ok := g.(map[string]interface{})
				if !ok {
					return resultErr("client_create: dict must be a map")
				}
				displayName, _ := m["displayName"].(string)
				var scopeList []string
				if v, ok := m["authorizationScopes"]; ok {
					switch arr := v.(type) {
					case []interface{}:
						for _, item := range arr {
							if s, ok := item.(string); ok {
								scopeList = append(scopeList, s)
							}
						}
					case []string:
						scopeList = arr
					}
				}
				if displayName == "" {
					return resultErr("client_create: missing displayName")
				}
				client, err := buddy.ClientCreateWithResult(c, scriptOpts, displayName, scopeList, logger)
				if err != nil {
					return resultErr(err.Error())
				}
				if client == nil {
					return resultErr("client_create: cancelled")
				}
				out := new(starlark.Dict)
				_ = out.SetKey(starlark.String("id"), starlark.String(strconv.Itoa(client.ID)))
				_ = out.SetKey(starlark.String("client_id"), starlark.String(client.ClientID))
				return resultOK(out), nil
			}
		}

		var displayName starlark.String
		var scopes *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("client_create", args, kwargs, "display_name", &displayName, "scopes", &scopes,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		var scopeList []string
		for i := 0; i < scopes.Len(); i++ {
			v := scopes.Index(i)
			switch s := v.(type) {
			case starlark.String:
				scopeList = append(scopeList, string(s))
			case starlark.Int:
				n, _ := s.Int64()
				scopeList = append(scopeList, strconv.FormatInt(n, 10))
			default:
				scopeList = append(scopeList, v.String())
			}
		}
		client, err := buddy.ClientCreateWithResult(c, scriptOpts, string(displayName), scopeList, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if client == nil {
			return resultErr("client_create: cancelled")
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(strconv.Itoa(client.ID)))
		_ = d.SetKey(starlark.String("client_id"), starlark.String(client.ClientID))
		return resultOK(d), nil
	}
}

func policySetScopeAll(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_set_scope_all", args, kwargs, "id", &policyID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		pid, _ := policyID.Int64()
		err := buddy.PolicyScopeAll(c, scriptOpts, int(pid), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func policyDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_delete", args, kwargs, "ids", &policyIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		var ids []int
		for i := 0; i < policyIDs.Len(); i++ {
			n, ok := policyIDs.Index(i).(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("policy_delete: ids must be a list of ints")
			}
			v, _ := n.Int64()
			ids = append(ids, int(v))
		}
		err := buddy.PolicyDelete(c, scriptOpts, ids, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// policy_put_xml(id, xml_str, tenant?, surface?, auth?)
func policyPutXML(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var xmlStr starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_put_xml", args, kwargs, "id", &policyID, "xml_str", &xmlStr,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		pid, _ := policyID.Int64()
		err := buddy.PolicyPutXML(c, scriptOpts, int(pid), []byte(string(xmlStr)), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func policyCopy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var name, trigger, categoryID starlark.String
		var matchTrigger, upsert starlark.Bool
		var scopeAll starlark.Value = starlark.None
		var scopeGroupIDs *starlark.List
		var dual dualSessionKwargs
		tail := append([]any{
			"id", &policyID, "name?", &name,
			"trigger?", &trigger,
			"match_trigger?", &matchTrigger,
			"category_id?", &categoryID,
			"upsert?", &upsert,
			"scope_all?", &scopeAll,
			"scope_group_ids?", &scopeGroupIDs,
		}, dual.unpackTail()...)
		if err := starlark.UnpackArgs("policy_copy", args, kwargs, tail...); err != nil {
			return nil, err
		}
		src, dst, err := dual.refs(thread, credName)
		if err != nil {
			return resultErr(err.Error())
		}
		pid, _ := policyID.Int64()
		groupIDs, scopeAllPtr, err := parseScopeKwargs(scopeAll, scopeGroupIDs)
		if err != nil {
			return resultErr(err.Error())
		}
		if bool(upsert) {
			migSpec := buddy.PolicyMigrateSpec{
				SourcePolicyID: int(pid),
				Name:           string(name),
				MatchTrigger:   bool(matchTrigger),
				CategoryID:     string(categoryID),
				ScopeAll:       scopeAllPtr,
				ScopeGroupIDs:  groupIDs,
			}
			if t := strings.TrimSpace(string(trigger)); t != "" {
				migSpec.Trigger = &t
			}
			out, err := buddy.PolicyMigrate(src, dst, scriptOpts, migSpec, logger)
			if err != nil {
				return resultErr(err.Error())
			}
			if out == nil {
				return resultErr("policy_copy: cancelled")
			}
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.MakeInt(out.PolicyID))
			_ = d.SetKey(starlark.String("name"), starlark.String(out.Name))
			_ = d.SetKey(starlark.String("trigger"), starlark.String(out.Trigger))
			_ = d.SetKey(starlark.String("created"), starlark.Bool(out.Created))
			return resultOK(d), nil
		}
		if strings.TrimSpace(string(name)) == "" {
			return resultErr("policy_copy: name is required unless upsert=True")
		}
		spec := buddy.PolicyCopySpec{
			SourcePolicyID: int(pid),
			Name:           string(name),
			CategoryID:     string(categoryID),
			ScopeAll:       scopeAllPtr,
			ScopeGroupIDs:  groupIDs,
		}
		if t := strings.TrimSpace(string(trigger)); t != "" {
			spec.Trigger = &t
		}
		out, err := buddy.PolicyCopy(src, dst, scriptOpts, spec, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("policy_copy: cancelled")
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.MakeInt(out.PolicyID))
		_ = d.SetKey(starlark.String("name"), starlark.String(out.Name))
		_ = d.SetKey(starlark.String("trigger"), starlark.String(out.Trigger))
		return resultOK(d), nil
	}
}

func policyMigrate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return policyCopy(credName, logger)(thread, b, args, withUpsertTrue(kwargs))
	}
}

func parseScopeKwargs(scopeAll starlark.Value, scopeGroupIDs *starlark.List) ([]int, *bool, error) {
	var groupIDs []int
	if scopeGroupIDs != nil {
		groupIDs = make([]int, 0, scopeGroupIDs.Len())
		for i := 0; i < scopeGroupIDs.Len(); i++ {
			n, ok := scopeGroupIDs.Index(i).(starlark.Int)
			if !ok {
				return nil, nil, fmt.Errorf("scope_group_ids must be a list of ints")
			}
			v64, _ := n.Int64()
			groupIDs = append(groupIDs, int(v64))
		}
	}
	var scopeAllPtr *bool
	if scopeAll != nil && scopeAll != starlark.None {
		b, ok := scopeAll.(starlark.Bool)
		if !ok {
			return nil, nil, fmt.Errorf("scope_all must be a bool")
		}
		v := bool(b)
		scopeAllPtr = &v
	}
	return groupIDs, scopeAllPtr, nil
}

// ---------------------------------------------------------------------------
// Package builtins
// ---------------------------------------------------------------------------

// package_put(id, dict, tenant?, surface?, auth?) → {ok, value: dict}
func packagePut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("package_put", args, kwargs, "id", &id, "dict", &bodyVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error())
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict")
		}
		updated, err := buddy.PackagePutFields(c, scriptOpts, string(id), m, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		b, mErr := json.Marshal(updated)
		if mErr != nil {
			return resultErr(mErr.Error())
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

// package_delete(ids=[...]) → {ok, value: None}
func packageDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var ids *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("package_delete", args, kwargs, "ids", &ids,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		var idStrs []string
		for i := 0; i < ids.Len(); i++ {
			v := ids.Index(i)
			switch tv := v.(type) {
			case starlark.String:
				idStrs = append(idStrs, string(tv))
			case starlark.Int:
				n, _ := tv.Int64()
				idStrs = append(idStrs, fmt.Sprintf("%d", n))
			default:
				return nil, fmt.Errorf("package_delete: ids must be a list of strings or ints")
			}
		}
		if err := buddy.PackageDelete(c, scriptOpts, idStrs, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func packageUpload(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var file, name, categoryID, notes, info starlark.String
		var priority starlark.Int
		var reboot starlark.Value = starlark.None
		var waitReady, force starlark.Bool
		var waitReadyTimeout starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("package_upload", args, kwargs,
			"file", &file,
			"name?", &name, "category_id?", &categoryID, "priority?", &priority,
			"notes?", &notes, "info?", &info, "reboot?", &reboot,
			"wait_ready?", &waitReady,
			"wait_ready_timeout?", &waitReadyTimeout,
			"force?", &force,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		var attrs buddy.PackageAttrs
		if name != "" {
			s := string(name)
			attrs.Name = &s
		}
		if categoryID != "" {
			s := string(categoryID)
			attrs.CategoryID = &s
		}
		if priority != starlark.MakeInt(0) {
			p64, _ := priority.Int64()
			p := int(p64)
			attrs.Priority = &p
		}
		if notes != "" {
			s := string(notes)
			attrs.Notes = &s
		}
		if info != "" {
			s := string(info)
			attrs.Info = &s
		}
		if reboot != nil && reboot != starlark.None {
			b, ok := reboot.(starlark.Bool)
			if !ok {
				return nil, fmt.Errorf("package_upload: reboot must be a bool")
			}
			v := bool(b)
			attrs.Reboot = &v
		}
		upOpts := buddy.PackageUploadOptions{WaitReady: bool(waitReady), Force: bool(force)}
		if waitReadyTimeout != nil && waitReadyTimeout != starlark.None {
			n, ok := waitReadyTimeout.(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("package_upload: wait_ready_timeout must be an int (seconds)")
			}
			sec, _ := n.Int64()
			if sec < 0 {
				return nil, fmt.Errorf("package_upload: wait_ready_timeout must be >= 0")
			}
			upOpts.WaitReady = true
			upOpts.WaitReadyTimeout = time.Duration(sec) * time.Second
		}
		err := buddy.PackageUpload(c, scriptOpts, string(file), attrs, upOpts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// package_ready(id=, tenant?, surface?, auth?) → {ok, value: bool}
func packageReady(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("package_ready", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		id, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		ready, err := buddy.PackageReady(c, id)
		if err != nil {
			return resultErr(err.Error())
		}
		_ = logger
		return resultOK(starlark.Bool(ready)), nil
	}
}

// package_wait_ready(id=, timeout?, tenant?, surface?, auth?) → {ok, value: None}
// timeout is seconds (default 600 = 10 minutes).
func packageWaitReady(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var timeout starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("package_wait_ready", args, kwargs, "id", &idVal, "timeout?", &timeout,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		id, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		var d time.Duration
		if timeout != nil && timeout != starlark.None {
			n, ok := timeout.(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("package_wait_ready: timeout must be an int (seconds)")
			}
			secs, _ := n.Int64()
			if secs < 0 {
				return nil, fmt.Errorf("package_wait_ready: timeout must be >= 0")
			}
			d = time.Duration(secs) * time.Second
		}
		if err := buddy.PackageWaitReady(c, id, d, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// packages_scan(class?, filter?, tenant?, surface?, auth?) → list of issue dicts
func packagesScanBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var class, filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("packages_scan", args, kwargs,
			"class?", &class,
			"filter?", &filter,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		classes, err := buddy.ParsePackageScanClasses(string(class))
		if err != nil {
			return nil, err
		}
		hits, err := buddy.PackageScan(sessionPin, string(filter), classes, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, h := range hits {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(h.ID))
			_ = d.SetKey(starlark.String("packageName"), starlark.String(h.PackageName))
			_ = d.SetKey(starlark.String("fileName"), starlark.String(h.FileName))
			_ = d.SetKey(starlark.String("cloudTransferStatus"), starlark.String(h.CloudTransferStatus))
			_ = d.SetKey(starlark.String("class"), starlark.String(h.Class))
			_ = d.SetKey(starlark.String("hashType"), starlark.String(h.HashType))
			_ = d.SetKey(starlark.String("hashValue"), starlark.String(h.HashValue))
			_ = d.SetKey(starlark.String("sha256"), starlark.String(h.Sha256))
			_ = d.SetKey(starlark.String("md5"), starlark.String(h.Md5))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// package_download(id=, path?, tenant?, surface?, auth?) → {ok, value: None}
func packageDownload(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var path starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("package_download", args, kwargs, "id", &idVal, "path?", &path,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		id, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		if err := buddy.PackageDownload(c, id, string(path), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// script_get_contents(id, tenant?, surface?, auth?) → {ok, value: contents_string}
func scriptGetContents(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_get_contents", args, kwargs, "id", &scriptID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		contents, err := buddy.ScriptGetContentsStr(c, string(scriptID))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(contents)), nil
	}
}

// script_create(name, script_contents, category_id?, priority?, info?, notes?, os_requirements?,
// parameter4?…parameter11?, tenant?, surface?, auth?) → dict (posted script + id)
func scriptCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, scriptContents, categoryID, priority, info, notes, osRequirements starlark.String
		var p4, p5, p6, p7, p8, p9, p10, p11 starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_create", args, kwargs,
			"name", &name,
			"script_contents", &scriptContents,
			"category_id?", &categoryID,
			"priority?", &priority,
			"info?", &info,
			"notes?", &notes,
			"os_requirements?", &osRequirements,
			"parameter4?", &p4,
			"parameter5?", &p5,
			"parameter6?", &p6,
			"parameter7?", &p7,
			"parameter8?", &p8,
			"parameter9?", &p9,
			"parameter10?", &p10,
			"parameter11?", &p11,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		pri := strings.TrimSpace(string(priority))
		if pri != "" {
			pri = strings.ToUpper(pri)
		}
		created, err := buddy.ScriptPostDetail(sessionPin, scriptOpts, buddy.ScriptDetail{
			Name:           string(name),
			ScriptContents: string(scriptContents),
			CategoryID:     strings.TrimSpace(string(categoryID)),
			Priority:       pri,
			Info:           string(info),
			Notes:          string(notes),
			OSRequirements: string(osRequirements),
			Parameter4:     string(p4),
			Parameter5:     string(p5),
			Parameter6:     string(p6),
			Parameter7:     string(p7),
			Parameter8:     string(p8),
			Parameter9:     string(p9),
			Parameter10:    string(p10),
			Parameter11:    string(p11),
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if created == nil {
			return resultErr("script_create: cancelled")
		}
		val, err := scriptDetailValue(created)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func scriptDetailValue(s *buddy.ScriptDetail) (starlark.Value, error) {
	if s == nil {
		return nil, fmt.Errorf("script detail is nil")
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return JSONDecodeToStarlark(string(b))
}

// script_update(id, file, tenant?, surface?, auth?) → {ok, value: None}
func scriptUpdate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID, file starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_update", args, kwargs, "id", &scriptID, "file", &file,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		if err := buddy.ScriptUpdate(c, scriptOpts, string(scriptID), string(file), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// script_delete(id=?) | script_delete(ids=?, tenant?, surface?, auth?) → None
func scriptDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var idsVal starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_delete", args, kwargs, "id?", &idVal, "ids?", &idsVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		idsSet := idsVal != nil && idsVal != starlark.None
		if err := xorIDOrKey("script_delete", idSet, idsSet, "ids"); err != nil {
			return nil, err
		}
		raw := idVal
		label := "id"
		if idsSet {
			raw = idsVal
			label = "ids"
		}
		ids, err := idListFromStarlark(raw, label)
		if err != nil {
			return nil, fmt.Errorf("script_delete: %w", err)
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		if err := buddy.ScriptDelete(sessionPin, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func groupDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("group_delete", args, kwargs, "id", &groupID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		err := buddy.GroupDelete(c, scriptOpts, string(groupID), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// error is like print but writes to stderr (same *args and optional sep= as print).
func errorBuiltin(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sep := " "
	if err := starlark.UnpackArgs("error", nil, kwargs, "sep?", &sep); err != nil {
		return nil, err
	}
	buf := new(strings.Builder)
	for i, v := range args {
		if i > 0 {
			buf.WriteString(sep)
		}
		appendPrintArg(buf, v)
	}
	fmt.Fprintln(writerFromLocal(thread, localStderr, os.Stderr), buf.String())
	return starlark.None, nil
}

func appendPrintArg(buf *strings.Builder, v starlark.Value) {
	if s, ok := starlark.AsString(v); ok {
		buf.WriteString(s)
	} else if b, ok := v.(starlark.Bytes); ok {
		buf.WriteString(string(b))
	} else {
		buf.WriteString(v.String())
	}
}

func confirmBuiltin(logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var msg, def, yes, no starlark.String
		if err := starlark.UnpackArgs("confirm", args, kwargs,
			"msg?", &msg,
			"default?", &def,
			"yes?", &yes,
			"no?", &no,
		); err != nil {
			return nil, err
		}
		if runnerOpts.SkipConfirm {
			return starlark.True, nil
		}
		if buddy.Confirm(buddy.ConfirmOpts{
			Question: string(msg),
			Default:  string(def),
			Yes:      string(yes),
			No:       string(no),
		}, logger) {
			return starlark.True, nil
		}
		return starlark.False, nil
	}
}

// auth(renew?=False, tenant?, surface?, auth?) → value: dict with current credential/session summary.
// renew=True forces a fresh token before querying Jamf.
func authBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var renew starlark.Bool
		var sess sessionArgs
		if err := starlark.UnpackArgs("auth", args, kwargs, "renew?", &renew,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		info, err := buddy.AuthInfo(c, bool(renew), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := GoValueToStarlark(info)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func exitBuiltin(logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var code starlark.Int = starlark.MakeInt(1)
		if err := starlark.UnpackArgs("exit", args, kwargs, "code?", &code); err != nil {
			return nil, err
		}
		c, _ := code.Int64()
		if inREPL && !inSession(thread) {
			os.Exit(int(c))
		}
		return nil, &ExitError{Code: int(c)}
	}
}

// ExitError is returned when the script calls exit(code).
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return "exit " + strconv.Itoa(e.Code)
}

// json_decode(json_str) → {ok, value: dict or list} or {ok: False, error: str}
func jsonDecodeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var jsonStr starlark.String
	if err := starlark.UnpackArgs("json_decode", args, kwargs, "json_str", &jsonStr); err != nil {
		return nil, err
	}
	val, err := JSONDecodeToStarlark(string(jsonStr))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(val), nil
}

// json_encode(value) → {ok, value: str} or {ok: False, error: str}
func jsonEncodeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var val starlark.Value
	if err := starlark.UnpackArgs("json_encode", args, kwargs, "value", &val); err != nil {
		return nil, err
	}
	s, err := StarlarkToJSON(val)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(s)), nil
}

// json_pretty(value) → {ok, value: str} or {ok: False, error: str}
func jsonPrettyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var val starlark.Value
	if err := starlark.UnpackArgs("json_pretty", args, kwargs, "value", &val); err != nil {
		return nil, err
	}
	s, err := StarlarkToJSONPretty(val)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(s)), nil
}

// plist_decode(plist_xml_str) → {ok, value: dict or list} or {ok: False, error: str}
func plistDecodeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr starlark.String
	if err := starlark.UnpackArgs("plist_decode", args, kwargs, "plist_xml_str", &plistStr); err != nil {
		return nil, err
	}
	val, err := PlistDecodeToStarlark(string(plistStr))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(val), nil
}

// plist_encode(value) → {ok, value: str} or {ok: False, error: str}
func plistEncodeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var val starlark.Value
	if err := starlark.UnpackArgs("plist_encode", args, kwargs, "value", &val); err != nil {
		return nil, err
	}
	s, err := StarlarkToPlist(val)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(s)), nil
}

// xml_escape(s) → {ok, value: str} — entity-encode for nesting inside XML element text.
func xmlEscapeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("xml_escape", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.XMLEscape(string(s)))), nil
}

// xml_unescape(s) → {ok, value: str} — reverse XML/HTML character entities.
func xmlUnescapeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("xml_unescape", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.XMLUnescape(string(s)))), nil
}

// xml_sort(xml_str, element?, key?, attr?, only?, order?, ignore_case?, numeric?, recursive?)
// → {ok, value: str}
func xmlSortBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var xmlStr, element, key, attr, only, order starlark.String
	ignoreCase := starlark.Bool(false)
	numeric := starlark.Bool(false)
	recursive := starlark.Bool(false)
	if err := starlark.UnpackArgs("xml_sort", args, kwargs,
		"xml_str", &xmlStr,
		"element?", &element,
		"key?", &key,
		"attr?", &attr,
		"only?", &only,
		"order?", &order,
		"ignore_case?", &ignoreCase,
		"numeric?", &numeric,
		"recursive?", &recursive,
	); err != nil {
		return nil, err
	}
	out, err := buddy.XMLSort(string(xmlStr), buddy.XMLSortOpts{
		Element:    string(element),
		Key:        string(key),
		Attr:       string(attr),
		Only:       string(only),
		Order:      string(order),
		IgnoreCase: bool(ignoreCase),
		Numeric:    bool(numeric),
		Recursive:  bool(recursive),
	})
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

// ---------------------------------------------------------------------------
// Version helpers (offline dotted-numeric compare / extract / sort)
// ---------------------------------------------------------------------------

// version_extract(s) → {ok, value: str}
func versionExtractBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("version_extract", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.ExtractVersion(string(s)))), nil
}

// version_compare(a, b) → {ok, value: int}  (-1, 0, 1)
func versionCompareBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, b starlark.String
	if err := starlark.UnpackArgs("version_compare", args, kwargs, "a", &a, "b", &b); err != nil {
		return nil, err
	}
	return resultOK(starlark.MakeInt(buddy.CompareVersions(string(a), string(b)))), nil
}

// version_sort(items, key?, order?) → {ok, value: list}
// items: list of str, or list of dict (key required for dicts).
func versionSortBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var items *starlark.List
	var key, order starlark.String
	if err := starlark.UnpackArgs("version_sort", args, kwargs, "items", &items, "key?", &key, "order?", &order); err != nil {
		return nil, err
	}
	if items == nil {
		return resultErr("items is required")
	}
	orderStr := strings.ToLower(strings.TrimSpace(string(order)))
	if orderStr == "" {
		orderStr = "asc"
	}
	if orderStr != "asc" && orderStr != "desc" {
		return resultErr(fmt.Sprintf("order must be \"asc\" or \"desc\", got %q", string(order)))
	}
	desc := orderStr == "desc"
	n := items.Len()
	if n == 0 {
		return resultOK(starlark.NewList(nil)), nil
	}

	vals := make([]starlark.Value, n)
	sortKeys := make([]string, n)
	keyName := string(key)
	for i := 0; i < n; i++ {
		v := items.Index(i)
		vals[i] = v
		switch tv := v.(type) {
		case starlark.String:
			if keyName != "" {
				return resultErr("key is only valid when items are dicts")
			}
			sortKeys[i] = string(tv)
		case *starlark.Dict:
			if keyName == "" {
				return resultErr("key is required when items are dicts")
			}
			fv, found, err := tv.Get(starlark.String(keyName))
			if err != nil {
				return resultErr(err.Error())
			}
			if !found {
				sortKeys[i] = ""
				continue
			}
			switch fv := fv.(type) {
			case starlark.String:
				sortKeys[i] = string(fv)
			case starlark.Int:
				sortKeys[i] = fv.String()
			default:
				sortKeys[i] = fv.String()
			}
		default:
			return resultErr("items must be a list of str or a list of dict")
		}
	}

	idxs := make([]int, n)
	for i := range idxs {
		idxs[i] = i
	}
	sort.SliceStable(idxs, func(i, j int) bool {
		return buddy.VersionSortLess(sortKeys[idxs[i]], sortKeys[idxs[j]], desc)
	})
	out := make([]starlark.Value, n)
	for i, idx := range idxs {
		out[i] = vals[idx]
	}
	return resultOK(starlark.NewList(out)), nil
}

// ---------------------------------------------------------------------------
// RSQL helpers (offline filter builders)
// ---------------------------------------------------------------------------

func rsqlEscapeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("rsql_escape", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.EscapeRSQLString(string(s)))), nil
}

func rsqlQuoteBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("rsql_quote", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.QuoteRSQLString(string(s)))), nil
}

func rsqlEqBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlCompareBuiltin("rsql_eq", args, kwargs, buddy.RSQLEq)
}

func rsqlNeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlCompareBuiltin("rsql_ne", args, kwargs, buddy.RSQLNe)
}

func rsqlLtBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlCompareBuiltin("rsql_lt", args, kwargs, buddy.RSQLLt)
}

func rsqlGtBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlCompareBuiltin("rsql_gt", args, kwargs, buddy.RSQLGt)
}

func rsqlLeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlCompareBuiltin("rsql_le", args, kwargs, buddy.RSQLLe)
}

func rsqlGeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlCompareBuiltin("rsql_ge", args, kwargs, buddy.RSQLGe)
}

func rsqlCompareBuiltin(name string, args starlark.Tuple, kwargs []starlark.Tuple, fn func(string, interface{}) (string, error)) (starlark.Value, error) {
	var field starlark.String
	var value starlark.Value
	if err := starlark.UnpackArgs(name, args, kwargs, "field", &field, "value", &value); err != nil {
		return nil, err
	}
	gv, err := rsqlStarlarkValue(value)
	if err != nil {
		return resultErr(err.Error())
	}
	out, err := fn(string(field), gv)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

func rsqlContainsBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var field, substring starlark.String
	if err := starlark.UnpackArgs("rsql_contains", args, kwargs, "field", &field, "substring", &substring); err != nil {
		return nil, err
	}
	out, err := buddy.RSQLContains(string(field), string(substring))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

func rsqlInBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var field starlark.String
	var values *starlark.List
	if err := starlark.UnpackArgs("rsql_in", args, kwargs, "field", &field, "values", &values); err != nil {
		return nil, err
	}
	items := make([]interface{}, 0, values.Len())
	for i := 0; i < values.Len(); i++ {
		gv, err := rsqlStarlarkValue(values.Index(i))
		if err != nil {
			return resultErr(fmt.Sprintf("values[%d]: %v", i, err))
		}
		items = append(items, gv)
	}
	out, err := buddy.RSQLIn(string(field), items)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

func rsqlAndBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlJoinBuiltin("rsql_and", args, kwargs, buddy.RSQLAnd)
}

func rsqlOrBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return rsqlJoinBuiltin("rsql_or", args, kwargs, buddy.RSQLOr)
}

func rsqlJoinBuiltin(name string, args starlark.Tuple, kwargs []starlark.Tuple, fn func([]string) (string, error)) (starlark.Value, error) {
	var clauses *starlark.List
	if err := starlark.UnpackArgs(name, args, kwargs, "clauses", &clauses); err != nil {
		return nil, err
	}
	parts := make([]string, 0, clauses.Len())
	for i := 0; i < clauses.Len(); i++ {
		s, ok := clauses.Index(i).(starlark.String)
		if !ok {
			return resultErr(fmt.Sprintf("clauses[%d]: must be a string", i))
		}
		parts = append(parts, string(s))
	}
	out, err := fn(parts)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

func rsqlDatetimeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var year, month, day starlark.Int
	hour, minute, second := starlark.MakeInt(0), starlark.MakeInt(0), starlark.MakeInt(0)
	if err := starlark.UnpackArgs("rsql_datetime", args, kwargs,
		"year", &year, "month", &month, "day", &day,
		"hour?", &hour, "minute?", &minute, "second?", &second); err != nil {
		return nil, err
	}
	y, _ := year.Int64()
	mo, _ := month.Int64()
	d, _ := day.Int64()
	h, _ := hour.Int64()
	mi, _ := minute.Int64()
	s, _ := second.Int64()
	out, err := buddy.RSQLDateTime(int(y), int(mo), int(d), int(h), int(mi), int(s))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

func rsqlNowBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("rsql_now", args, kwargs); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.RSQLNow())), nil
}

func rsqlDaysAgoBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var days starlark.Value
	if err := starlark.UnpackArgs("rsql_days_ago", args, kwargs, "days", &days); err != nil {
		return nil, err
	}
	n, err := rsqlNumber(days)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(buddy.RSQLDaysAgo(n))), nil
}

func rsqlHoursAgoBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var hours starlark.Value
	if err := starlark.UnpackArgs("rsql_hours_ago", args, kwargs, "hours", &hours); err != nil {
		return nil, err
	}
	n, err := rsqlNumber(hours)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(buddy.RSQLHoursAgo(n))), nil
}

func rsqlStarlarkValue(v starlark.Value) (interface{}, error) {
	switch x := v.(type) {
	case starlark.String:
		return string(x), nil
	case starlark.Bool:
		return bool(x), nil
	case starlark.Int:
		n, ok := x.Int64()
		if !ok {
			return nil, fmt.Errorf("integer out of range")
		}
		return n, nil
	case starlark.Float:
		return float64(x), nil
	case starlark.NoneType:
		return nil, fmt.Errorf("value is None")
	default:
		return nil, fmt.Errorf("unsupported value type %s (want str, int, float, or bool)", v.Type())
	}
}

func rsqlNumber(v starlark.Value) (float64, error) {
	switch x := v.(type) {
	case starlark.Int:
		n, ok := x.Int64()
		if !ok {
			return 0, fmt.Errorf("integer out of range")
		}
		return float64(n), nil
	case starlark.Float:
		return float64(x), nil
	default:
		return 0, fmt.Errorf("want int or float, got %s", v.Type())
	}
}

// file_read(path) → str — UTF-8 text only; use file_read_bytes for binary.
func fileReadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	if err := starlark.UnpackArgs("file_read", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if mcpGuarded() && buddySecretPath(string(path)) {
		return resultErr(errMCPSecretPath().Error())
	}
	b, err := os.ReadFile(string(path))
	if err != nil {
		return resultErr(err.Error())
	}
	if !utf8.Valid(b) {
		return resultErr("file is not valid UTF-8; use file_read_bytes for binary data")
	}
	return resultOK(starlark.String(string(b))), nil
}

// file_read_bytes(path) → {ok, value: bytes}
func fileReadBytesBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	if err := starlark.UnpackArgs("file_read_bytes", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if mcpGuarded() && buddySecretPath(string(path)) {
		return resultErr(errMCPSecretPath().Error())
	}
	b, err := os.ReadFile(string(path))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.Bytes(b)), nil
}

// file_write(path, data) → None — data is str (UTF-8) or bytes.
func fileWriteBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	var data starlark.Value
	if err := starlark.UnpackArgs("file_write", args, kwargs, "path", &path, "data", &data); err != nil {
		return nil, err
	}
	var raw []byte
	switch d := data.(type) {
	case starlark.String:
		raw = []byte(string(d))
	case starlark.Bytes:
		raw = []byte(d)
	default:
		return resultErr("data must be str or bytes")
	}
	if err := os.WriteFile(string(path), raw, 0o644); err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.None), nil
}

// ---------------------------------------------------------------------------
// Declarative workflow: step constructor builtins
// Each returns a Starlark dict describing the step; no side effects.
// ---------------------------------------------------------------------------

func attachStepNote(d *starlark.Dict, note starlark.String) *starlark.Dict {
	if string(note) != "" {
		_ = d.SetKey(starlark.String("note"), note)
	}
	return d
}

func stepNoteBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var note starlark.String
	if err := starlark.UnpackArgs("step_note", args, kwargs, "note", &note); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(note)) == "" {
		return nil, fmt.Errorf("step_note requires non-empty note")
	}
	return stepDict(stepTypeNote, "note", string(note)), nil
}

func stepWithNoteBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var step *starlark.Dict
	var note starlark.String
	if err := starlark.UnpackArgs("step_with_note", args, kwargs, "step", &step, "note", &note); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(note)) == "" {
		return nil, fmt.Errorf("step_with_note requires non-empty note")
	}
	_ = step.SetKey(starlark.String("note"), note)
	return step, nil
}

func changeBatchBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var title, summary, risk, window starlark.String
	if err := starlark.UnpackArgs("change_batch", args, kwargs,
		"title", &title,
		"summary?", &summary,
		"risk?", &risk,
		"window?", &window,
	); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(title)) == "" {
		return nil, fmt.Errorf("change_batch requires title")
	}
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("type"), starlark.String("change_batch"))
	_ = d.SetKey(starlark.String("title"), title)
	if string(summary) != "" {
		_ = d.SetKey(starlark.String("summary"), summary)
	}
	if string(risk) != "" {
		_ = d.SetKey(starlark.String("risk"), risk)
	}
	if string(window) != "" {
		_ = d.SetKey(starlark.String("window"), window)
	}
	return d, nil
}

func stepSetPackageBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, version, fromTrigger, packagePattern, note starlark.String
	var policyID starlark.Int
	disableSource := starlark.Value(starlark.None)
	keepSource := starlark.Value(starlark.None)
	if err := starlark.UnpackArgs("step_set_package", args, kwargs,
		"name", &name, "version", &version, "from_trigger", &fromTrigger,
		"package_pattern?", &packagePattern,
		"id?", &policyID,
		"disable_source?", &disableSource, "keep_source?", &keepSource,
		"note?", &note,
	); err != nil {
		return nil, err
	}
	if _, err := disableModeFromPackageKwargs(disableSource, keepSource); err != nil {
		return nil, err
	}
	d := stepDict(stepTypeSetPackage,
		"name", string(name),
		"version", string(version),
		"from_trigger", string(fromTrigger),
	)
	if pid, _ := policyID.Int64(); pid > 0 {
		_ = d.SetKey(starlark.String("id"), policyID)
	}
	if string(packagePattern) != "" {
		_ = d.SetKey(starlark.String("package_pattern"), packagePattern)
	}
	if disableSource != nil && disableSource != starlark.None {
		_ = d.SetKey(starlark.String("disable_source"), disableSource)
	}
	if keepSource != nil && keepSource != starlark.None {
		_ = d.SetKey(starlark.String("keep_source"), keepSource)
	}
	return attachStepNote(d, note), nil
}

func stepClonePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fromTrigger, version, name, oldVersion, packagePattern, note starlark.String
	var params *starlark.List
	keepSource := starlark.Value(starlark.None)
	if err := starlark.UnpackArgs("step_clone_policy", args, kwargs,
		"from_trigger", &fromTrigger, "version", &version,
		"name?", &name, "old_version?", &oldVersion, "package_pattern?", &packagePattern, "params?", &params,
		"keep_source?", &keepSource,
		"note?", &note,
	); err != nil {
		return nil, err
	}
	if _, err := disableModeFromKeepSource(keepSource); err != nil {
		return nil, err
	}
	d := stepDict(stepTypeClonePolicy,
		"from_trigger", string(fromTrigger),
		"version", string(version),
	)
	if string(name) != "" {
		_ = d.SetKey(starlark.String("name"), name)
	}
	if string(oldVersion) != "" {
		_ = d.SetKey(starlark.String("old_version"), oldVersion)
	}
	if string(packagePattern) != "" {
		_ = d.SetKey(starlark.String("package_pattern"), packagePattern)
	}
	if params != nil {
		_ = d.SetKey(starlark.String("params"), params)
	}
	if keepSource != nil && keepSource != starlark.None {
		_ = d.SetKey(starlark.String("keep_source"), keepSource)
	}
	return attachStepNote(d, note), nil
}

func stepSetSSBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var version starlark.String
	var policyID starlark.Int
	var groupIDs *starlark.List
	if err := starlark.UnpackArgs("step_set_ss", args, kwargs,
		"id", &policyID, "version", &version, "group_ids?", &groupIDs); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	d := stepDict(stepTypeSetSS,
		"id", int(pid),
		"version", string(version),
	)
	if groupIDs != nil {
		_ = d.SetKey(starlark.String("group_ids"), groupIDs)
	}
	return d, nil
}

func stepSetGroupVersionBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var groupID, version starlark.String
	if err := starlark.UnpackArgs("step_set_group_version", args, kwargs,
		"id", &groupID, "version", &version); err != nil {
		return nil, err
	}
	return stepDict(stepTypeSetGroupVersion,
		"id", string(groupID),
		"version", string(version),
	), nil
}

func stepSetInstallBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var version starlark.String
	var policyID starlark.Int
	var params *starlark.List
	if err := starlark.UnpackArgs("step_set_install", args, kwargs,
		"id", &policyID, "version", &version, "params?", &params); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	d := stepDict(stepTypeSetInstall,
		"id", int(pid),
		"version", string(version),
	)
	if params != nil {
		_ = d.SetKey(starlark.String("params"), params)
	}
	return d, nil
}

func stepEnablePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_enable_policy", args, kwargs, "id", &policyID); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeEnablePolicy, "id", int(pid)), nil
}

func stepDisablePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_disable_policy", args, kwargs, "id", &policyID); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeDisablePolicy, "id", int(pid)), nil
}

func stepSetTriggerBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var trigger starlark.String
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_set_trigger", args, kwargs,
		"id", &policyID, "trigger", &trigger); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeSetTrigger,
		"id", int(pid),
		"trigger", string(trigger),
	), nil
}

func stepSetScopeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	var groupIDs *starlark.List
	if err := starlark.UnpackArgs("step_set_scope", args, kwargs,
		"id", &policyID, "group_ids", &groupIDs); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	d := stepDict(stepTypeSetScope, "id", int(pid))
	_ = d.SetKey(starlark.String("group_ids"), groupIDs)
	return d, nil
}

func stepSetScopeAllBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_set_scope_all", args, kwargs, "id", &policyID); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeSetScopeAll, "id", int(pid)), nil
}

func stepDeletePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyIDs *starlark.List
	if err := starlark.UnpackArgs("step_delete_policy", args, kwargs, "ids", &policyIDs); err != nil {
		return nil, err
	}
	d := stepDict(stepTypeDeletePolicy)
	_ = d.SetKey(starlark.String("ids"), policyIDs)
	return d, nil
}

// policy_template returns a Starlark dict describing a policy template.
// The sentinel key "__policy_template__" distinguishes it from step dicts.
func policyTemplateBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, trigger starlark.String
	var scriptID starlark.Int
	var scriptPriority starlark.String
	var params *starlark.Dict
	var frequency, category, distributionPoint starlark.String
	var scopeGroupIDs *starlark.List
	var scopeAll starlark.Bool
	var recon starlark.Bool

	if err := starlark.UnpackArgs("policy_template", args, kwargs,
		"name", &name,
		"trigger", &trigger,
		"script_id", &scriptID,
		"script_priority?", &scriptPriority,
		"params?", &params,
		"frequency?", &frequency,
		"category?", &category,
		"scope_group_ids?", &scopeGroupIDs,
		"scope_all?", &scopeAll,
		"recon?", &recon,
		"distribution_point?", &distributionPoint,
	); err != nil {
		return nil, err
	}

	sid, _ := scriptID.Int64()

	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("__policy_template__"), starlark.True)
	_ = d.SetKey(starlark.String("name"), name)
	_ = d.SetKey(starlark.String("trigger"), trigger)
	_ = d.SetKey(starlark.String("script_id"), starlark.MakeInt64(sid))
	_ = d.SetKey(starlark.String("script_priority"), scriptPriority)
	if params != nil {
		_ = d.SetKey(starlark.String("params"), params)
	} else {
		_ = d.SetKey(starlark.String("params"), new(starlark.Dict))
	}
	_ = d.SetKey(starlark.String("frequency"), frequency)
	_ = d.SetKey(starlark.String("category"), category)
	if scopeGroupIDs != nil {
		_ = d.SetKey(starlark.String("scope_group_ids"), scopeGroupIDs)
	} else {
		_ = d.SetKey(starlark.String("scope_group_ids"), starlark.NewList(nil))
	}
	_ = d.SetKey(starlark.String("scope_all"), scopeAll)
	_ = d.SetKey(starlark.String("recon"), recon)
	_ = d.SetKey(starlark.String("distribution_point"), distributionPoint)
	return d, nil
}

// step_create_policy builds a workflow step dict from a policy_template dict and a version string.
// {version} placeholders in the template's string fields are substituted at execution time
// (inside stepFromStarlark, so planLine shows post-substitution values).
func stepCreatePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var tmpl *starlark.Dict
	var version starlark.String
	if err := starlark.UnpackArgs("step_create_policy", args, kwargs,
		"template", &tmpl,
		"version", &version,
	); err != nil {
		return nil, err
	}

	// Verify it is a policy template dict.
	sentinel, _, _ := tmpl.Get(starlark.String("__policy_template__"))
	if sentinel != starlark.True {
		return nil, fmt.Errorf("step_create_policy: template must be a dict returned by policy_template()")
	}

	// Build the step dict: copy all template fields plus type and version.
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("type"), starlark.String(stepTypeCreatePolicy))
	_ = d.SetKey(starlark.String("version"), version)
	for _, item := range tmpl.Items() {
		k := item[0]
		if k == starlark.String("__policy_template__") {
			continue
		}
		_ = d.SetKey(k, item[1])
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// Profile builtins
// ---------------------------------------------------------------------------

// profile_get_payloads(profile_id) → {ok, value: str} (payloads plist XML)
func profileGetPayloads(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_get_payloads", args, kwargs, "id", &profileID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		pid, _ := profileID.Int64()
		var buf bytes.Buffer
		if err := buddy.ProfileGetPayloads(sessionPin, int(pid), &buf, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(buf.String())), nil
	}
}

func payloadContentRowsStarlark(rows []buddy.PayloadContentRow) *starlark.List {
	list := new(starlark.List)
	for _, r := range rows {
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("identifier"), starlark.String(r.Identifier))
		_ = d.SetKey(starlark.String("type"), starlark.String(r.Type))
		_ = d.SetKey(starlark.String("uuid"), starlark.String(r.UUID))
		_ = list.Append(d)
	}
	return list
}

// profile_get_payload(id, identifier) → {ok, value: str} (single payload plist XML)
func profileGetPayload(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var identifier starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_get_payload", args, kwargs, "id", &profileID, "identifier", &identifier,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		pid, _ := profileID.Int64()
		var buf bytes.Buffer
		if err := buddy.ProfileGetPayloadByDomain(sessionPin, int(pid), string(identifier), &buf, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(buf.String())), nil
	}
}

// profile_list_payloads(id) → {ok, value: list of {identifier, type, uuid}}
func profileListPayloads(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_list_payloads", args, kwargs, "id", &profileID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		pid, _ := profileID.Int64()
		rows, err := buddy.ProfileListPayloads(sessionPin, int(pid), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(payloadContentRowsStarlark(rows)), nil
	}
}

// profile_redeploy(profile_id, tenant?, surface?, auth?) → {ok, value: None}
func profileRedeploy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_redeploy", args, kwargs,
			"id", &profileID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		pid, _ := profileID.Int64()
		err := buddy.ProfileRedeploy(sessionPin, scriptOpts, int(pid), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// profile_acs_bind_uuid(profile_id, payload_uuid) → None
// Seeds the process cache so profile reads can hydrate ACS Forced.plist when Jamf's
// legacy HTML does not expose FIELD_EXTERNAL_CONFIGPROFILE_ID to API-token clients.
func profileACSBindUUIDBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var profileID starlark.Int
	var payloadUUID starlark.String
	if err := starlark.UnpackArgs("profile_acs_bind_uuid", args, kwargs,
		"id", &profileID,
		"payload_uuid", &payloadUUID,
	); err != nil {
		return nil, err
	}
	pid, _ := profileID.Int64()
	u := strings.TrimSpace(string(payloadUUID))
	if pid <= 0 || u == "" {
		return resultErr("profile_id and payload_uuid are required")
	}
	buddy.CacheACSPayloadUUID(int(pid), u)
	return resultOK(starlark.None), nil
}

func profileSetPayload(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var identifier, dictFile starlark.String
		redeploy := starlark.Bool(false)
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_set_payload", args, kwargs,
			"id", &profileID,
			"identifier", &identifier,
			"dict_file", &dictFile,
			"redeploy?", &redeploy,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		pid, _ := profileID.Int64()
		err := buddy.ProfileSetPayload(sessionPin, scriptOpts, int(pid), string(identifier), string(dictFile), bool(redeploy), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func profileSetXML(logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var xmlFile starlark.String
		redeploy := starlark.Bool(false)
		if err := starlark.UnpackArgs("profile_set_xml", args, kwargs,
			"id", &profileID,
			"xml_file", &xmlFile,
			"redeploy?", &redeploy,
		); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		f, err := os.Open(string(xmlFile))
		if err != nil {
			return resultErr(fmt.Sprintf("open %s: %v", string(xmlFile), err))
		}
		defer f.Close()
		cred := getAuth(thread, "")
		err = buddy.ProfileSetXML(cred, scriptOpts, int(pid), f, bool(redeploy), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func profileCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, payloadsPlist, category, redeployOnUpdate starlark.String
		var content *starlark.List
		var scopeAll starlark.Value = starlark.None
		var scopeGroupIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_create", args, kwargs,
			"name", &name,
			"payloads_plist_str?", &payloadsPlist,
			"content?", &content,
			"category?", &category,
			"scope_all?", &scopeAll,
			"scope_group_ids?", &scopeGroupIDs,
			"redeploy_on_update?", &redeployOnUpdate,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		plistStr := string(payloadsPlist)
		if content != nil {
			items := make([]interface{}, 0, content.Len())
			for i := 0; i < content.Len(); i++ {
				g, err := StarlarkValueToGo(content.Index(i))
				if err != nil {
					return resultErr(fmt.Sprintf("content[%d]: %v", i, err))
				}
				items = append(items, g)
			}
			updated, err := buddy.PlistSetPayloadContent(plistStr, items)
			if err != nil {
				return resultErr(err.Error())
			}
			plistStr = updated
		}
		if strings.TrimSpace(plistStr) == "" {
			return resultErr("profile_create requires payloads_plist_str or content")
		}
		var groupIDs []int
		if scopeGroupIDs != nil {
			for i := 0; i < scopeGroupIDs.Len(); i++ {
				n, ok := scopeGroupIDs.Index(i).(starlark.Int)
				if !ok {
					return resultErr("scope_group_ids must be a list of ints")
				}
				v64, _ := n.Int64()
				groupIDs = append(groupIDs, int(v64))
			}
		}
		all := len(groupIDs) == 0
		if scopeAll != nil && scopeAll != starlark.None {
			b, ok := scopeAll.(starlark.Bool)
			if !ok {
				return resultErr("scope_all must be a bool")
			}
			all = bool(b)
		}
		out, err := buddy.ProfileCreate(c, scriptOpts, buddy.ProfileCreateSpec{
			Name:             string(name),
			PayloadsPlist:    plistStr,
			Category:         string(category),
			ScopeAll:         all,
			ScopeGroupIDs:    groupIDs,
			RedeployOnUpdate: string(redeployOnUpdate),
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("profile_create: cancelled")
		}
		return resultOK(starlark.MakeInt(out.ProfileID)), nil
	}
}

func profileCreateOutcomeValue(out *buddy.ProfileCreateOutcome) (starlark.Value, error) {
	if out == nil {
		return resultErr("profile create: cancelled")
	}
	d := starlark.NewDict(2)
	_ = d.SetKey(starlark.String("id"), starlark.MakeInt(out.ProfileID))
	_ = d.SetKey(starlark.String("name"), starlark.String(out.Name))
	return resultOK(d), nil
}

func uuidNewBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("uuid_new", args, kwargs); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.NewUUID())), nil
}

func uuidIsBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("uuid_is", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return resultOK(starlark.Bool(buddy.IsUUID(string(s)))), nil
}

func profileClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return profileCopy(credName, logger)(thread, b, args, kwargs)
	}
}

func profileCopy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var name, categoryID starlark.String
		var scopeAll starlark.Value = starlark.None
		var scopeGroupIDs *starlark.List
		upsert := starlark.Bool(false)
		var dual dualSessionKwargs
		tail := append([]any{
			"id", &profileID, "name?", &name,
			"category_id?", &categoryID,
			"scope_all?", &scopeAll,
			"scope_group_ids?", &scopeGroupIDs,
			"upsert?", &upsert,
		}, dual.unpackTail()...)
		if err := starlark.UnpackArgs("profile_copy", args, kwargs, tail...); err != nil {
			return nil, err
		}
		src, dst, err := dual.refs(thread, credName)
		if err != nil {
			return resultErr(err.Error())
		}
		pid, _ := profileID.Int64()
		groupIDs, scopeAllPtr, err := parseScopeKwargs(scopeAll, scopeGroupIDs)
		if err != nil {
			return resultErr(err.Error())
		}
		if bool(upsert) {
			out, err := buddy.ProfileMigrate(src, dst, scriptOpts, buddy.ProfileMigrateSpec{
				SourceProfileID: int(pid),
				Name:            string(name),
				ScopeAll:        scopeAllPtr,
				ScopeGroupIDs:   groupIDs,
				CategoryID:      string(categoryID),
			}, logger)
			if err != nil {
				return resultErr(err.Error())
			}
			if out == nil {
				return resultErr("profile_copy: cancelled")
			}
			d := starlark.NewDict(3)
			_ = d.SetKey(starlark.String("id"), starlark.MakeInt(out.ProfileID))
			_ = d.SetKey(starlark.String("name"), starlark.String(out.Name))
			_ = d.SetKey(starlark.String("created"), starlark.Bool(out.Created))
			return resultOK(d), nil
		}
		if strings.TrimSpace(string(name)) == "" {
			return resultErr("profile_copy: name is required unless upsert=True")
		}
		out, err := buddy.ProfileCopy(src, dst, scriptOpts, buddy.ProfileCopySpec{
			SourceProfileID: int(pid),
			Name:            string(name),
			ScopeAll:        scopeAllPtr,
			ScopeGroupIDs:   groupIDs,
			CategoryID:      string(categoryID),
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return profileCreateOutcomeValue(out)
	}
}

func profileMigrate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return profileCopy(credName, logger)(thread, b, args, withUpsertTrue(kwargs))
	}
}

func profileCreateXML(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var xmlStr, profileName starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_create_xml", args, kwargs,
			"xml_str", &xmlStr, "name?", &profileName,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		name := string(profileName)
		if name == "" {
			nameStart := strings.Index(string(xmlStr), "<name>")
			if nameStart >= 0 {
				rest := string(xmlStr)[nameStart+len("<name>"):]
				if end := strings.Index(rest, "</name>"); end >= 0 {
					name = rest[:end]
				}
			}
		}
		out, err := buddy.ProfilePostXML(sessionPin, scriptOpts, []byte(string(xmlStr)), name, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return profileCreateOutcomeValue(out)
	}
}

func profileDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_delete", args, kwargs, "id", &profileID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		pid, _ := profileID.Int64()
		if err := buddy.ProfileDelete(sessionPin, scriptOpts, int(pid), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// profile_put_xml(profile_id, xml_str, redeploy?, tenant?, surface?, auth?)
func profilePutXML(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var xmlStr starlark.String
		redeploy := starlark.Bool(false)
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_put_xml", args, kwargs,
			"id", &profileID, "xml_str", &xmlStr, "redeploy?", &redeploy,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		pid, _ := profileID.Int64()
		credResolved := c
		err := buddy.ProfilePutXML(credResolved, scriptOpts, int(pid), []byte(string(xmlStr)), bool(redeploy), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// profile_xml_extract_payloads(profile_xml_str) → {ok, value: str}
func profileXMLExtractPayloadsBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var xmlStr starlark.String
	if err := starlark.UnpackArgs("profile_xml_extract_payloads", args, kwargs, "profile_xml_str", &xmlStr); err != nil {
		return nil, err
	}
	payloads, err := buddy.ExtractPayloadsFromProfileXML([]byte(string(xmlStr)))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(payloads)), nil
}

// profile_xml_randomize_uuids(profile_xml_str) → {ok, value: str}
func profileXMLRandomizeUUIDsBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var xmlStr starlark.String
	if err := starlark.UnpackArgs("profile_xml_randomize_uuids", args, kwargs, "profile_xml_str", &xmlStr); err != nil {
		return nil, err
	}
	out, err := buddy.RandomizeProfileUUIDs([]byte(string(xmlStr)))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(string(out))), nil
}

// profile_xml_prepare_for_create(profile_xml_str, name, scope_all?, scope_group_ids?) → {ok, value: str}
func profileXMLPrepareForCreateBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var xmlStr, name starlark.String
	var scopeAll starlark.Value = starlark.None
	var scopeGroupIDs *starlark.List
	if err := starlark.UnpackArgs("profile_xml_prepare_for_create", args, kwargs,
		"profile_xml_str", &xmlStr, "name", &name,
		"scope_all?", &scopeAll,
		"scope_group_ids?", &scopeGroupIDs); err != nil {
		return nil, err
	}
	spec := buddy.ProfilePrepareSpec{Name: string(name)}
	if scopeGroupIDs != nil {
		ids := make([]int, 0, scopeGroupIDs.Len())
		for i := 0; i < scopeGroupIDs.Len(); i++ {
			n, ok := scopeGroupIDs.Index(i).(starlark.Int)
			if !ok {
				return resultErr("scope_group_ids must be a list of ints")
			}
			v64, _ := n.Int64()
			ids = append(ids, int(v64))
		}
		spec.ScopeGroupIDs = ids
	}
	if scopeAll != starlark.None {
		b, ok := scopeAll.(starlark.Bool)
		if !ok {
			return resultErr("scope_all must be a bool")
		}
		v := bool(b)
		spec.ScopeAll = &v
	}
	out, err := buddy.PrepareProfileXMLForCreate([]byte(string(xmlStr)), spec)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(string(out))), nil
}

// profile_xml_replace_payloads(profile_xml_str, payloads_plist_str) → {ok, value: str}
func profileXMLReplacePayloadsBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var profileXMLStr, payloadsStr starlark.String
	if err := starlark.UnpackArgs("profile_xml_replace_payloads", args, kwargs,
		"profile_xml_str", &profileXMLStr, "payloads_plist_str", &payloadsStr); err != nil {
		return nil, err
	}
	out, err := buddy.ReplacePayloadsContent([]byte(string(profileXMLStr)), string(payloadsStr))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(string(out))), nil
}

// profile_xml_set_redeploy_all(profile_xml_str) → {ok, value: str}
func profileXMLSetRedeployAllBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var xmlStr starlark.String
	if err := starlark.UnpackArgs("profile_xml_set_redeploy_all", args, kwargs, "profile_xml_str", &xmlStr); err != nil {
		return nil, err
	}
	out := buddy.SetRedeployAll([]byte(string(xmlStr)))
	return resultOK(starlark.String(string(out))), nil
}

// profile_xml_patch_payload(profile_xml_str, identifier, new_payload) → {ok, value: str}
// new_payload is a Starlark dict or plist/dict XML string (PayloadContent or MCX settings).
func profileXMLPatchPayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var xmlStr, identifier starlark.String
	var newPayload starlark.Value
	if err := starlark.UnpackArgs("profile_xml_patch_payload", args, kwargs,
		"profile_xml_str", &xmlStr, "identifier", &identifier, "new_payload", &newPayload); err != nil {
		return nil, err
	}
	dictXML, err := payloadValueToDictXML(newPayload)
	if err != nil {
		return resultErr(err.Error())
	}
	updated, err := buddy.PatchProfilePayload([]byte(string(xmlStr)), string(identifier), dictXML)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(string(updated))), nil
}

// plist_normalise_dict(data_str) → {ok, value: str}  (bare <dict>…</dict>)
func plistNormaliseDictBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data starlark.String
	if err := starlark.UnpackArgs("plist_normalise_dict", args, kwargs, "data_str", &data); err != nil {
		return nil, err
	}
	out, err := buddy.NormaliseDictXML([]byte(string(data)))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

// payloads_list_identifiers(payloads_plist_str) → {ok, value: list of {identifier, type, uuid}}
func payloadsListIdentifiersBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr starlark.String
	if err := starlark.UnpackArgs("payloads_list_identifiers", args, kwargs, "payloads_plist_str", &plistStr); err != nil {
		return nil, err
	}
	rows, err := buddy.PlistPayloadContentRows(string(plistStr))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(payloadContentRowsStarlark(rows)), nil
}

// payloads_collect_uuids(payloads_plist_str) → {ok, value: list of str}
func payloadsCollectUUIDsBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr starlark.String
	if err := starlark.UnpackArgs("payloads_collect_uuids", args, kwargs, "payloads_plist_str", &plistStr); err != nil {
		return nil, err
	}
	uuids, err := buddy.PlistCollectPayloadUUIDs(string(plistStr))
	if err != nil {
		return resultErr(err.Error())
	}
	list := new(starlark.List)
	for _, u := range uuids {
		_ = list.Append(starlark.String(u))
	}
	return resultOK(list), nil
}

// payloads_get_payload(payloads_plist_str, identifier) → {ok, value: str}
func payloadsGetPayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr, identifier starlark.String
	if err := starlark.UnpackArgs("payloads_get_payload", args, kwargs, "payloads_plist_str", &plistStr, "identifier", &identifier); err != nil {
		return nil, err
	}
	out, err := buddy.PlistGetPayloadByDomain(string(plistStr), string(identifier))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(string(out))), nil
}

// payloads_replace_payload(payloads_plist_str, identifier, new_payload) → {ok, value: str}. new_payload is dict or plist XML string.
func payloadsReplacePayload(logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var plistStr, identifier starlark.String
		var newPayload starlark.Value
		if err := starlark.UnpackArgs("payloads_replace_payload", args, kwargs,
			"payloads_plist_str", &plistStr, "identifier", &identifier, "new_payload", &newPayload); err != nil {
			return nil, err
		}
		var payloadMap interface{}
		switch v := newPayload.(type) {
		case *starlark.Dict:
			g, err := StarlarkValueToGo(v)
			if err != nil {
				return resultErr(err.Error())
			}
			payloadMap = g
		case starlark.String:
			decoded, err := PlistDecodeToStarlark(string(v))
			if err != nil {
				return resultErr(err.Error())
			}
			d, ok := decoded.(*starlark.Dict)
			if !ok {
				return resultErr("new_payload plist string must decode to a dict")
			}
			g, err := StarlarkValueToGo(d)
			if err != nil {
				return resultErr(err.Error())
			}
			payloadMap = g
		default:
			return resultErr("new_payload must be dict or plist XML string")
		}
		updated, err := buddy.PlistReplacePayloadByDomain(string(plistStr), string(identifier), payloadMap)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(updated)), nil
	}
}

// payloads_replace_mcx_dict(payloads_plist_str, domain, dict_xml_str) → {ok, value: str}
// dict_xml_str may be a bare <dict>…</dict> or a full plist from plist_encode (normalised before splice).
func payloadsReplaceMCXDictBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr, domain, dictXML starlark.String
	if err := starlark.UnpackArgs("payloads_replace_mcx_dict", args, kwargs,
		"payloads_plist_str", &plistStr, "domain", &domain, "dict_xml_str", &dictXML); err != nil {
		return nil, err
	}
	updated, err := buddy.ReplaceMCXDict(string(plistStr), string(domain), string(dictXML))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(updated)), nil
}

// payloads_get_mcx_dict(payloads_plist_str, domain) → {ok, value: str} bare <dict> for mcx_preference_settings
func payloadsGetMCXDictBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr, domain starlark.String
	if err := starlark.UnpackArgs("payloads_get_mcx_dict", args, kwargs,
		"payloads_plist_str", &plistStr, "domain", &domain); err != nil {
		return nil, err
	}
	out, err := buddy.GetMCXDict(string(plistStr), string(domain))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(out)), nil
}

// payloads_set_content(content, payloads_plist_str?) → {ok, value: str}
// content is a list of payload dicts. Empty/omitted payloads_plist_str starts a new plist.
func payloadsSetContentBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var content *starlark.List
	var plistStr starlark.String
	if err := starlark.UnpackArgs("payloads_set_content", args, kwargs,
		"content", &content, "payloads_plist_str?", &plistStr); err != nil {
		return nil, err
	}
	items := make([]interface{}, 0, content.Len())
	for i := 0; i < content.Len(); i++ {
		g, err := StarlarkValueToGo(content.Index(i))
		if err != nil {
			return resultErr(fmt.Sprintf("content[%d]: %v", i, err))
		}
		items = append(items, g)
	}
	updated, err := buddy.PlistSetPayloadContent(string(plistStr), items)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(updated)), nil
}

// payloads_add_payload(payloads_plist_str, identifier, new_payload) → {ok, value: str}
// Appends or replaces by identifier=. Empty payloads_plist_str creates a new plist.
func payloadsAddPayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr, identifier starlark.String
	var newPayload starlark.Value
	if err := starlark.UnpackArgs("payloads_add_payload", args, kwargs,
		"payloads_plist_str", &plistStr, "identifier", &identifier, "new_payload", &newPayload); err != nil {
		return nil, err
	}
	payloadMap, err := payloadValueToGoMap(newPayload)
	if err != nil {
		return resultErr(err.Error())
	}
	updated, err := buddy.PlistAddPayload(string(plistStr), string(identifier), payloadMap)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(updated)), nil
}

// payloads_remove_payload(payloads_plist_str, identifier) → {ok, value: str}
func payloadsRemovePayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr, identifier starlark.String
	if err := starlark.UnpackArgs("payloads_remove_payload", args, kwargs,
		"payloads_plist_str", &plistStr, "identifier", &identifier); err != nil {
		return nil, err
	}
	updated, err := buddy.PlistRemovePayload(string(plistStr), string(identifier))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(updated)), nil
}

// payloadValueToGoMap accepts a Starlark dict or plist XML string and returns a Go map.
func payloadValueToGoMap(newPayload starlark.Value) (map[string]interface{}, error) {
	switch v := newPayload.(type) {
	case *starlark.Dict:
		g, err := StarlarkValueToGo(v)
		if err != nil {
			return nil, err
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("new_payload dict did not convert to a map")
		}
		return m, nil
	case starlark.String:
		decoded, err := PlistDecodeToStarlark(string(v))
		if err != nil {
			return nil, err
		}
		d, ok := decoded.(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("new_payload plist string must decode to a dict")
		}
		g, err := StarlarkValueToGo(d)
		if err != nil {
			return nil, err
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("new_payload dict did not convert to a map")
		}
		return m, nil
	default:
		return nil, fmt.Errorf("new_payload must be dict or plist XML string")
	}
}

// payloadValueToDictXML encodes a Starlark dict or normalises a plist/dict XML string to a bare <dict> fragment.
func payloadValueToDictXML(newPayload starlark.Value) (string, error) {
	switch v := newPayload.(type) {
	case *starlark.Dict:
		encoded, err := StarlarkToPlist(v)
		if err != nil {
			return "", err
		}
		return buddy.NormaliseDictXML([]byte(encoded))
	case starlark.String:
		return buddy.NormaliseDictXML([]byte(string(v)))
	default:
		return "", fmt.Errorf("new_payload must be dict or plist/dict XML string")
	}
}

func stepSetProfilePayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var profileID starlark.Int
	var identifier, dictFile starlark.String
	redeploy := starlark.Bool(false)
	if err := starlark.UnpackArgs("step_set_profile_payload", args, kwargs,
		"id", &profileID,
		"identifier", &identifier,
		"dict_file", &dictFile,
		"redeploy?", &redeploy,
	); err != nil {
		return nil, err
	}
	pid, _ := profileID.Int64()
	d := stepDict(stepTypeSetProfilePayload,
		"id", int(pid),
		"identifier", string(identifier),
		"dict_file", string(dictFile),
		"redeploy", bool(redeploy),
	)
	return d, nil
}

func stepSetProfileXMLBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var profileID starlark.Int
	var xmlFile starlark.String
	redeploy := starlark.Bool(false)
	if err := starlark.UnpackArgs("step_set_profile_xml", args, kwargs,
		"id", &profileID,
		"xml_file", &xmlFile,
		"redeploy?", &redeploy,
	); err != nil {
		return nil, err
	}
	pid, _ := profileID.Int64()
	d := stepDict(stepTypeSetProfileXML,
		"id", int(pid),
		"xml_file", string(xmlFile),
		"redeploy", bool(redeploy),
	)
	return d, nil
}

func stepCreatePackagePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, version, action, category, packagePattern, note starlark.String
	var scopeAll starlark.Value = starlark.None
	var scopeGroupIDs *starlark.List
	if err := starlark.UnpackArgs("step_create_package_policy", args, kwargs,
		"name", &name, "version", &version,
		"action?", &action,
		"scope_all?", &scopeAll,
		"scope_group_ids?", &scopeGroupIDs,
		"category?", &category,
		"package_pattern?", &packagePattern,
		"note?", &note,
	); err != nil {
		return nil, err
	}
	act := string(action)
	if act == "" {
		act = "install"
	}
	d := stepDict(stepTypeCreatePackagePolicy,
		"name", string(name),
		"version", string(version),
		"action", act,
	)
	if scopeAll != nil && scopeAll != starlark.None {
		_ = d.SetKey(starlark.String("scope_all"), scopeAll)
	}
	if scopeGroupIDs != nil {
		_ = d.SetKey(starlark.String("scope_group_ids"), scopeGroupIDs)
	}
	if string(category) != "" {
		_ = d.SetKey(starlark.String("category"), category)
	}
	if string(packagePattern) != "" {
		_ = d.SetKey(starlark.String("package_pattern"), packagePattern)
	}
	return attachStepNote(d, note), nil
}

func stepCreateProfileBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, payloadsPlist, category, redeployOnUpdate, note starlark.String
	var content *starlark.List
	var scopeAll starlark.Value = starlark.None
	var scopeGroupIDs *starlark.List
	if err := starlark.UnpackArgs("step_create_profile", args, kwargs,
		"name", &name,
		"payloads_plist_str?", &payloadsPlist,
		"content?", &content,
		"category?", &category,
		"scope_all?", &scopeAll,
		"scope_group_ids?", &scopeGroupIDs,
		"redeploy_on_update?", &redeployOnUpdate,
		"note?", &note,
	); err != nil {
		return nil, err
	}
	plistStr := string(payloadsPlist)
	if content != nil {
		items := make([]interface{}, 0, content.Len())
		for i := 0; i < content.Len(); i++ {
			g, err := StarlarkValueToGo(content.Index(i))
			if err != nil {
				return nil, fmt.Errorf("content[%d]: %v", i, err)
			}
			items = append(items, g)
		}
		updated, err := buddy.PlistSetPayloadContent(plistStr, items)
		if err != nil {
			return nil, err
		}
		plistStr = updated
	}
	if strings.TrimSpace(plistStr) == "" {
		return nil, fmt.Errorf("step_create_profile requires payloads_plist_str or content")
	}
	d := stepDict(stepTypeCreateProfile,
		"name", string(name),
		"payloads_plist_str", plistStr,
	)
	if scopeAll != nil && scopeAll != starlark.None {
		_ = d.SetKey(starlark.String("scope_all"), scopeAll)
	}
	if scopeGroupIDs != nil {
		_ = d.SetKey(starlark.String("scope_group_ids"), scopeGroupIDs)
	}
	if string(category) != "" {
		_ = d.SetKey(starlark.String("category"), category)
	}
	if string(redeployOnUpdate) != "" {
		_ = d.SetKey(starlark.String("redeploy_on_update"), redeployOnUpdate)
	}
	return attachStepNote(d, note), nil
}

func stepCreateSSBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var trigger, displayName, category, note starlark.String
	var scopeGroupID starlark.Int = starlark.MakeInt(0)
	var scriptID starlark.Int = starlark.MakeInt(0)
	if err := starlark.UnpackArgs("step_create_ss", args, kwargs,
		"trigger", &trigger, "display_name", &displayName,
		"category?", &category,
		"scope_group_id?", &scopeGroupID,
		"script_id?", &scriptID,
		"note?", &note,
	); err != nil {
		return nil, err
	}
	sgid, _ := scopeGroupID.Int64()
	sid, _ := scriptID.Int64()
	d := stepDict(stepTypeCreateSS,
		"trigger", string(trigger),
		"display_name", string(displayName),
		"scope_group_id", int(sgid),
		"script_id", int(sid),
	)
	if string(category) != "" {
		_ = d.SetKey(starlark.String("category"), category)
	}
	return attachStepNote(d, note), nil
}

func pppcFDABuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var bundleID, codeRequirement, comment starlark.String
	if err := starlark.UnpackArgs("pppc_fda", args, kwargs,
		"bundle_id", &bundleID, "code_requirement", &codeRequirement, "comment?", &comment,
	); err != nil {
		return nil, err
	}
	entry := buddy.PPPCFDA(string(bundleID), string(codeRequirement), string(comment))
	v, err := GoValueToStarlark(entry)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(v), nil
}

func pppcPayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var services *starlark.Dict
	var payloadIdentifier, payloadUUID, displayName starlark.String
	if err := starlark.UnpackArgs("pppc_payload", args, kwargs,
		"services", &services,
		"payload_identifier?", &payloadIdentifier,
		"payload_uuid?", &payloadUUID,
		"payload_display_name?", &displayName,
	); err != nil {
		return nil, err
	}
	g, err := StarlarkValueToGo(services)
	if err != nil {
		return resultErr(err.Error())
	}
	svcMap, ok := g.(map[string]interface{})
	if !ok {
		return resultErr("services must be a dict")
	}
	payload, err := buddy.PPPCPayload(svcMap, buddy.PayloadMetaOpts{
		PayloadIdentifier:  string(payloadIdentifier),
		PayloadUUID:        string(payloadUUID),
		PayloadDisplayName: string(displayName),
	})
	if err != nil {
		return resultErr(err.Error())
	}
	v, err := GoValueToStarlark(payload)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(v), nil
}

func prefsPayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var domain starlark.String
	var settings *starlark.Dict
	var payloadIdentifier, payloadUUID, displayName starlark.String
	if err := starlark.UnpackArgs("prefs_payload", args, kwargs,
		"domain", &domain,
		"settings", &settings,
		"payload_identifier?", &payloadIdentifier,
		"payload_uuid?", &payloadUUID,
		"payload_display_name?", &displayName,
	); err != nil {
		return nil, err
	}
	var settingsMap map[string]interface{}
	if settings != nil {
		g, err := StarlarkValueToGo(settings)
		if err != nil {
			return resultErr(err.Error())
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("settings must be a dict")
		}
		settingsMap = m
	}
	payload, err := buddy.PrefsPayload(string(domain), settingsMap, buddy.PayloadMetaOpts{
		PayloadIdentifier:  string(payloadIdentifier),
		PayloadUUID:        string(payloadUUID),
		PayloadDisplayName: string(displayName),
	})
	if err != nil {
		return resultErr(err.Error())
	}
	v, err := GoValueToStarlark(payload)
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(v), nil
}

// script_put(id, dict, tenant?, surface?, auth?) → {ok, value: dict}
func scriptPutBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID starlark.String
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_put", args, kwargs, "id", &scriptID, "dict", &bodyVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error())
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict")
		}
		updated, err := buddy.ScriptPutFields(c, scriptOpts, string(scriptID), m, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		b, mErr := json.Marshal(updated)
		if mErr != nil {
			return resultErr(mErr.Error())
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

// ---------------------------------------------------------------------------
// Computer builtins
// ---------------------------------------------------------------------------

// computer_reboot(computer_ids, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerReboot(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_reboot", args, kwargs, "ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_reboot")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerReboot(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_redeploy(credName, computer_ids) → {ok, value: None} or {ok: False, error: "..."}
func computerRedeploy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_redeploy", args, kwargs, "ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_redeploy")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerRedeploy(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func parseComputerIDsAndCred(computerIDs *starlark.List, authPin string, fn string) ([]int, string, error) {
	var ids []int
	for i := 0; i < computerIDs.Len(); i++ {
		v := computerIDs.Index(i)
		n, ok := v.(starlark.Int)
		if !ok {
			return nil, "", fmt.Errorf("%s: computer_ids must be a list of ints", fn)
		}
		n64, _ := n.Int64()
		ids = append(ids, int(n64))
	}
	return ids, authPin, nil
}

// computer_send_mdm_command(computer_ids, command_type, command_data?, tenant?, surface?, auth?) → {ok, value: None}
func computerSendMDMCommand(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var commandType starlark.String
		var commandDataVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_send_mdm_command", args, kwargs,
			"ids", &computerIDs,
			"command_type", &commandType,
			"command_data?", &commandDataVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_send_mdm_command")
		if err != nil {
			return nil, err
		}
		var cmdData map[string]interface{}
		if commandDataVal != nil && commandDataVal != starlark.None {
			g, err := StarlarkValueToGo(commandDataVal)
			if err != nil {
				return resultErr(err.Error())
			}
			m, ok := g.(map[string]interface{})
			if !ok {
				return resultErr("command_data must be a dict")
			}
			cmdData = m
		}
		if err := buddy.ComputerSendMDMCommand(c, ids, string(commandType), cmdData, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_shutdown(computer_ids, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerShutdown(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_shutdown", args, kwargs, "ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_shutdown")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerShutdown(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_lock(computer_ids, message?, phone?, pin?, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerLock(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var message, phone, pin starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_lock", args, kwargs,
			"ids", &computerIDs,
			"message?", &message, "phone?", &phone, "pin?", &pin,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_lock")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerLock(c, scriptOpts, ids, string(message), string(phone), string(pin), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_logout(computer_ids, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerLogout(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_logout", args, kwargs, "ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_logout")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerLogOut(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_erase(computer_ids, pin?, obliteration?, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerErase(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var pin, obliteration starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_erase", args, kwargs,
			"ids", &computerIDs, "pin?", &pin, "obliteration?", &obliteration,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_erase")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerErase(c, scriptOpts, ids, string(pin), string(obliteration), false, false, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_device_info(computer_ids, queries?, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerDeviceInfo(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var queries *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_device_info", args, kwargs,
			"ids", &computerIDs, "queries?", &queries,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_device_info")
		if err != nil {
			return nil, err
		}
		var q []string
		if queries != nil && queries.Len() > 0 {
			for i := 0; i < queries.Len(); i++ {
				v := queries.Index(i)
				if s, ok := v.(starlark.String); ok {
					q = append(q, string(s))
				}
			}
		}
		if err := buddy.ComputerDeviceInfo(c, scriptOpts, ids, q, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_device_location(computer_ids, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerDeviceLocation(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_device_location", args, kwargs, "ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_device_location")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerDeviceLocation(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_security_info(computer_ids, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerSecurityInfo(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_security_info", args, kwargs, "ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_security_info")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerSecurityInfo(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_installed_apps(computer_ids, identifiers?, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerInstalledApps(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var identifiers *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_installed_apps", args, kwargs,
			"ids", &computerIDs, "identifiers?", &identifiers,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_installed_apps")
		if err != nil {
			return nil, err
		}
		var idList []string
		if identifiers != nil && identifiers.Len() > 0 {
			for i := 0; i < identifiers.Len(); i++ {
				v := identifiers.Index(i)
				if s, ok := v.(starlark.String); ok {
					idList = append(idList, string(s))
				}
			}
		}
		if err := buddy.ComputerInstalledApps(c, scriptOpts, ids, idList, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_managed_apps(computer_ids, identifiers?, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerManagedApps(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var identifiers *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_managed_apps", args, kwargs,
			"ids", &computerIDs, "identifiers?", &identifiers,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_managed_apps")
		if err != nil {
			return nil, err
		}
		var idList []string
		if identifiers != nil && identifiers.Len() > 0 {
			for i := 0; i < identifiers.Len(); i++ {
				v := identifiers.Index(i)
				if s, ok := v.(starlark.String); ok {
					idList = append(idList, string(s))
				}
			}
		}
		if err := buddy.ComputerManagedApps(c, scriptOpts, ids, idList, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// computer_certificates(computer_ids, tenant?, surface?, auth?) → {ok, value: None} or {ok: False, error: "..."}
func computerCertificates(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_certificates", args, kwargs, "ids", &computerIDs,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		ids, c, err := parseComputerIDsAndCred(computerIDs, c, "computer_certificates")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerCertificates(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// ---------------------------------------------------------------------------
// MDM builtins
// ---------------------------------------------------------------------------

// mdm_command_find(filter, sort?, page?, page_size?, limit?, tenant?, surface?, auth?) → {ok, value: list of dicts}
// Each dict has keys: uuid, command_type, command_state, client_management_id,
// client_type, date_sent, date_completed.
// Omit page to walk all pages; page_size is per request; limit caps total when page is omitted (0 = no cap).
func mdmCommandFind(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sort starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var limit starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("mdm_command_find", args, kwargs,
			"filter", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize, "limit?", &limit,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		sortStr := string(sort)
		if sortStr == "" {
			sortStr = "dateSent:asc"
		}
		lim, _ := limit.Int64()
		records, err := buddy.MdmCommandFind(c, string(filter), sortStr, opts, int(lim), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, r := range records {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("uuid"), starlark.String(r.UUID))
			_ = d.SetKey(starlark.String("command_type"), starlark.String(r.CommandType))
			_ = d.SetKey(starlark.String("command_state"), starlark.String(r.CommandState))
			_ = d.SetKey(starlark.String("client_management_id"), starlark.String(r.Client.ManagementID))
			_ = d.SetKey(starlark.String("client_type"), starlark.String(r.Client.ClientType))
			_ = d.SetKey(starlark.String("date_sent"), starlark.String(r.DateSent))
			_ = d.SetKey(starlark.String("date_completed"), starlark.String(r.DateCompleted))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// mdm_fetch_commands(filter, sort?, page?, page_size?, limit?, tenant?, surface?, auth?) → {ok, value: list of dicts}
// Alias of mdm_command_find with a more \"component\"-style name.
func mdmFetchCommands(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sort starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var limit starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("mdm_fetch_commands", args, kwargs,
			"filter", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize, "limit?", &limit,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		sortStr := string(sort)
		if sortStr == "" {
			sortStr = "dateSent:asc"
		}
		lim, _ := limit.Int64()
		records, err := buddy.MdmCommandFind(c, string(filter), sortStr, opts, int(lim), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, r := range records {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("uuid"), starlark.String(r.UUID))
			_ = d.SetKey(starlark.String("command_type"), starlark.String(r.CommandType))
			_ = d.SetKey(starlark.String("command_state"), starlark.String(r.CommandState))
			_ = d.SetKey(starlark.String("client_management_id"), starlark.String(r.Client.ManagementID))
			_ = d.SetKey(starlark.String("client_type"), starlark.String(r.Client.ClientType))
			_ = d.SetKey(starlark.String("date_sent"), starlark.String(r.DateSent))
			_ = d.SetKey(starlark.String("date_completed"), starlark.String(r.DateCompleted))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// ---------------------------------------------------------------------------
// run_workflow / change-batch builtins
// ---------------------------------------------------------------------------

func runWorkflowBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var steps *starlark.List
		if err := starlark.UnpackArgs("run_workflow", args, kwargs, "steps", &steps); err != nil {
			return nil, err
		}
		cred := credName
		stepVals := make([]starlark.Value, steps.Len())
		for i := 0; i < steps.Len(); i++ {
			stepVals[i] = steps.Index(i)
		}
		if err := runWorkflow(cred, stepVals, logger); err != nil {
			return nil, err
		}
		return starlark.None, nil
	}
}

func planChangeBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var change starlark.Value
		var steps *starlark.List
		if err := starlark.UnpackArgs("plan_change", args, kwargs, "change", &change, "steps", &steps); err != nil {
			return nil, err
		}
		cred := credName
		stepVals := make([]starlark.Value, steps.Len())
		for i := 0; i < steps.Len(); i++ {
			stepVals[i] = steps.Index(i)
		}
		md, fps, err := planChange(cred, change, stepVals, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		fpList := starlark.NewList(nil)
		for _, fp := range fps {
			_ = fpList.Append(starlark.String(fp))
		}
		val := new(starlark.Dict)
		_ = val.SetKey(starlark.String("markdown"), starlark.String(md))
		_ = val.SetKey(starlark.String("fingerprints"), fpList)
		fmt.Print(md)
		return resultOK(val), nil
	}
}

func runChangeBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var change starlark.Value
		var steps *starlark.List
		planned := starlark.Value(starlark.None)
		if err := starlark.UnpackArgs("run_change", args, kwargs,
			"change", &change, "steps", &steps, "planned?", &planned,
		); err != nil {
			return nil, err
		}
		cred := credName
		stepVals := make([]starlark.Value, steps.Len())
		for i := 0; i < steps.Len(); i++ {
			stepVals[i] = steps.Index(i)
		}
		var fps []string
		if planned != nil && planned != starlark.None {
			pd, ok := planned.(*starlark.Dict)
			if !ok {
				return nil, fmt.Errorf("planned must be the dict returned by plan_change")
			}
			// Accept either the raw plan_change value dict or a result wrapper.
			if v, found, _ := pd.Get(starlark.String("value")); found {
				if inner, ok := v.(*starlark.Dict); ok {
					pd = inner
				}
			}
			fpVal, found, err := pd.Get(starlark.String("fingerprints"))
			if err != nil {
				return nil, err
			}
			if found {
				lst, ok := fpVal.(*starlark.List)
				if !ok {
					return nil, fmt.Errorf("planned.fingerprints must be a list")
				}
				for i := 0; i < lst.Len(); i++ {
					s, ok := lst.Index(i).(starlark.String)
					if !ok {
						return nil, fmt.Errorf("planned.fingerprints[%d] must be a string", i)
					}
					fps = append(fps, string(s))
				}
			}
		}
		md, err := runChange(cred, change, stepVals, fps, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(md)), nil
	}
}
