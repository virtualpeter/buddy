package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
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

func resultOK(value starlark.Value) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("ok"), starlark.Bool(true))
	_ = d.SetKey(starlark.String("value"), value)
	return d
}

func resultErr(msg string) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("ok"), starlark.Bool(false))
	_ = d.SetKey(starlark.String("error"), starlark.String(msg))
	return d
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

func makeBuiltins(credName string, logger *slog.Logger, trace bool) starlark.StringDict {
	dict := starlark.StringDict{
		// Imperative builtins (backward-compatible).
		"policy_get_id":                      starlark.NewBuiltin("policy_get_id", policyGetID(credName, logger)),
		"policy_get_enable":                  starlark.NewBuiltin("policy_get_enable", policyGetEnable(credName, logger)),
		"policy_get_details":                 starlark.NewBuiltin("policy_get_details", policyGetDetails(credName, logger)),
		"policy_set_ss":                      starlark.NewBuiltin("policy_set_ss", policySetSS(credName, logger)),
		"policy_set_enable":                  starlark.NewBuiltin("policy_set_enable", policySetEnable(credName, logger)),
		"policy_set_disable":                 starlark.NewBuiltin("policy_set_disable", policySetDisable(credName, logger)),
		"policy_set_trigger":                 starlark.NewBuiltin("policy_set_trigger", policySetTrigger(credName, logger)),
		"policy_set_name":                    starlark.NewBuiltin("policy_set_name", policySetName(credName, logger)),
		"policy_set_frequency":               starlark.NewBuiltin("policy_set_frequency", policySetFrequency(credName, logger)),
		"policy_set_install":                 starlark.NewBuiltin("policy_set_install", policySetInstall(credName, logger)),
		"policy_clone":                       starlark.NewBuiltin("policy_clone", policyClone(credName, logger)),
		"policy_create_ss":                   starlark.NewBuiltin("policy_create_ss", policyCreateSS(credName, logger)),
		"policy_set_package":                 starlark.NewBuiltin("policy_set_package", policySetPackage(credName, logger)),
		"policy_set_scope_all":               starlark.NewBuiltin("policy_set_scope_all", policySetScopeAll(credName, logger)),
		"policy_delete":                      starlark.NewBuiltin("policy_delete", policyDelete(credName, logger)),
		"policy_find_ids":                    starlark.NewBuiltin("policy_find_ids", policyFindIDs(credName, logger)),
		"policy_fetch_xml":                   starlark.NewBuiltin("policy_fetch_xml", policyFetchXML(credName, logger)),
		"policy_put_xml":                     starlark.NewBuiltin("policy_put_xml", policyPutXML(credName, logger)),
		"group_list_smart":                   starlark.NewBuiltin("group_list_smart", groupListSmart(credName, logger)),
		"group_list_static":                  starlark.NewBuiltin("group_list_static", groupListStatic(credName, logger)),
		"group_find_id":                      starlark.NewBuiltin("group_find_id", groupFindID(credName, logger)),
		"group_get_version":                  starlark.NewBuiltin("group_get_version", groupGetVersion(credName, logger)),
		"group_set_version":                  starlark.NewBuiltin("group_set_version", groupSetVersion(credName, logger)),
		"smart_group_fetch":                  starlark.NewBuiltin("smart_group_fetch", smartGroupFetch(credName, logger)),
		"smart_group_put":                    starlark.NewBuiltin("smart_group_put", smartGroupPut(credName, logger)),
		"smart_group_get_criteria":           starlark.NewBuiltin("smart_group_get_criteria", smartGroupGetCriteria(credName, logger)),
		"smart_group_set_criteria":           starlark.NewBuiltin("smart_group_set_criteria", smartGroupSetCriteria(credName, logger)),
		"smart_group_find_criteria":          starlark.NewBuiltin("smart_group_find_criteria", smartGroupFindCriteria(credName, logger)),
		"smart_group_update_criterion_value": starlark.NewBuiltin("smart_group_update_criterion_value", smartGroupUpdateCriterionValue(credName, logger)),
		"group_delete":                       starlark.NewBuiltin("group_delete", groupDelete(credName, logger)),
		"static_group_list_computers":        starlark.NewBuiltin("static_group_list_computers", staticGroupListComputers(credName, logger)),
		"static_group_add_computers":         starlark.NewBuiltin("static_group_add_computers", staticGroupAddComputers(credName, logger)),
		"static_group_remove_computers":      starlark.NewBuiltin("static_group_remove_computers", staticGroupRemoveComputers(credName, logger)),
		"static_group_set_computers":         starlark.NewBuiltin("static_group_set_computers", staticGroupSetComputers(credName, logger)),
		"group_fetch":                        starlark.NewBuiltin("group_fetch", groupFetch(credName, logger)),
		"group_put":                          starlark.NewBuiltin("group_put", groupPut(credName, logger)),
		"role_privileges_list":               starlark.NewBuiltin("role_privileges_list", rolePrivilegesList(credName, logger)),
		"role_list":                          starlark.NewBuiltin("role_list", roleList(credName, logger)),
		"role_fetch_list":                    starlark.NewBuiltin("role_fetch_list", roleFetchList(credName, logger)),
		"role_fetch":                         starlark.NewBuiltin("role_fetch", roleFetch(credName, logger)),
		"role_put":                           starlark.NewBuiltin("role_put", rolePut(credName, logger)),
		"role_create":                        starlark.NewBuiltin("role_create", roleCreate(credName, logger)),
		"client_fetch_list":                  starlark.NewBuiltin("client_fetch_list", clientFetchList(credName, logger)),
		"client_fetch":                       starlark.NewBuiltin("client_fetch", clientFetch(credName, logger)),
		"client_put":                         starlark.NewBuiltin("client_put", clientPut(credName, logger)),
		"client_create":                      starlark.NewBuiltin("client_create", clientCreate(credName, logger)),
		"error":                              starlark.NewBuiltin("error", errorBuiltin),
		"exit":                               starlark.NewBuiltin("exit", exitBuiltin(logger)),

		// Shared: JSON/plist decode and encode (component primitives).
		"json_decode":  starlark.NewBuiltin("json_decode", jsonDecodeBuiltin),
		"json_encode":  starlark.NewBuiltin("json_encode", jsonEncodeBuiltin),
		"plist_decode": starlark.NewBuiltin("plist_decode", plistDecodeBuiltin),
		"plist_encode": starlark.NewBuiltin("plist_encode", plistEncodeBuiltin),

		"file_read":       starlark.NewBuiltin("file_read", fileReadBuiltin),
		"file_read_bytes": starlark.NewBuiltin("file_read_bytes", fileReadBytesBuiltin),
		"file_write":      starlark.NewBuiltin("file_write", fileWriteBuiltin),

		// Declarative workflow: step constructors.
		"step_set_package":       starlark.NewBuiltin("step_set_package", stepSetPackageBuiltin),
		"step_clone_policy":      starlark.NewBuiltin("step_clone_policy", stepClonePolicyBuiltin),
		"step_set_ss":            starlark.NewBuiltin("step_set_ss", stepSetSSBuiltin),
		"step_set_group_version": starlark.NewBuiltin("step_set_group_version", stepSetGroupVersionBuiltin),
		"step_set_install":       starlark.NewBuiltin("step_set_install", stepSetInstallBuiltin),
		"step_enable_policy":     starlark.NewBuiltin("step_enable_policy", stepEnablePolicyBuiltin),
		"step_disable_policy":    starlark.NewBuiltin("step_disable_policy", stepDisablePolicyBuiltin),
		"step_set_trigger":       starlark.NewBuiltin("step_set_trigger", stepSetTriggerBuiltin),
		"step_set_scope":         starlark.NewBuiltin("step_set_scope", stepSetScopeBuiltin),
		"step_set_scope_all":     starlark.NewBuiltin("step_set_scope_all", stepSetScopeAllBuiltin),
		"step_delete_policy":     starlark.NewBuiltin("step_delete_policy", stepDeletePolicyBuiltin),
		"policy_template":        starlark.NewBuiltin("policy_template", policyTemplateBuiltin),
		"step_create_policy":     starlark.NewBuiltin("step_create_policy", stepCreatePolicyBuiltin),

		// Declarative workflow: executor.
		"run_workflow": starlark.NewBuiltin("run_workflow", runWorkflowBuiltin(credName, logger)),

		// Package: imperative builtins.
		"package_list":    starlark.NewBuiltin("package_list", packageList(credName, logger)),
		"package_find_id": starlark.NewBuiltin("package_find_id", packageFindID(credName, logger)),
		"package_upload":  starlark.NewBuiltin("package_upload", packageUpload(credName, logger)),
		"package_delete":  starlark.NewBuiltin("package_delete", packageDelete(credName, logger)),
		"package_fetch":   starlark.NewBuiltin("package_fetch", packageFetch(credName, logger)),
		"package_put":     starlark.NewBuiltin("package_put", packagePut(credName, logger)),

		// Profile: imperative builtins.
		"profile_list":          starlark.NewBuiltin("profile_list", profileList(credName, logger)),
		"profile_find_id":       starlark.NewBuiltin("profile_find_id", profileFindID(credName, logger)),
		"profile_clone":         starlark.NewBuiltin("profile_clone", profileClone(credName, logger)),
		"profile_redeploy":      starlark.NewBuiltin("profile_redeploy", profileRedeploy(credName, logger)),
		"profile_get_payloads":  starlark.NewBuiltin("profile_get_payloads", profileGetPayloads(credName, logger)),
		"profile_get_payload":   starlark.NewBuiltin("profile_get_payload", profileGetPayload(credName, logger)),
		"profile_list_payloads": starlark.NewBuiltin("profile_list_payloads", profileListPayloads(credName, logger)),
		"profile_set_payload":   starlark.NewBuiltin("profile_set_payload", profileSetPayload(logger)),
		"profile_set_xml":       starlark.NewBuiltin("profile_set_xml", profileSetXML(logger)),

		// Profile: component builtins (fetch/put, XML and payloads primitives).
		"profile_fetch_xml":            starlark.NewBuiltin("profile_fetch_xml", profileFetchXML(credName, logger)),
		"profile_put_xml":              starlark.NewBuiltin("profile_put_xml", profilePutXML(credName, logger)),
		"profile_xml_extract_payloads": starlark.NewBuiltin("profile_xml_extract_payloads", profileXMLExtractPayloadsBuiltin),
		"profile_xml_replace_payloads": starlark.NewBuiltin("profile_xml_replace_payloads", profileXMLReplacePayloadsBuiltin),
		"profile_xml_set_redeploy_all": starlark.NewBuiltin("profile_xml_set_redeploy_all", profileXMLSetRedeployAllBuiltin),
		"payloads_list_identifiers":    starlark.NewBuiltin("payloads_list_identifiers", payloadsListIdentifiersBuiltin),
		"payloads_get_payload":         starlark.NewBuiltin("payloads_get_payload", payloadsGetPayloadBuiltin),
		"payloads_replace_payload":     starlark.NewBuiltin("payloads_replace_payload", payloadsReplacePayload(logger)),
		"payloads_replace_mcx_dict":    starlark.NewBuiltin("payloads_replace_mcx_dict", payloadsReplaceMCXDictBuiltin),

		// Profile: declarative step constructors.
		"step_set_profile_payload": starlark.NewBuiltin("step_set_profile_payload", stepSetProfilePayloadBuiltin),
		"step_set_profile_xml":     starlark.NewBuiltin("step_set_profile_xml", stepSetProfileXMLBuiltin),

		// Script (Jamf Pro scripts): imperative builtins.
		"script_list":         starlark.NewBuiltin("script_list", scriptList(credName, logger)),
		"script_get_id":       starlark.NewBuiltin("script_get_id", scriptGetID(credName, logger)),
		"script_get_contents": starlark.NewBuiltin("script_get_contents", scriptGetContents(credName, logger)),
		"script_update":       starlark.NewBuiltin("script_update", scriptUpdate(credName, logger)),
		"script_fetch":        starlark.NewBuiltin("script_fetch", scriptFetch(credName, logger)),
		"script_put":          starlark.NewBuiltin("script_put", scriptPutBuiltin(credName, logger)),

		// Computer: imperative builtins.
		"computer_find_ids":         starlark.NewBuiltin("computer_find_ids", computerFindIDs(credName, logger)),
		"computer_get":              starlark.NewBuiltin("computer_get", computerGet(credName, logger)),
		"computer_fetch":            starlark.NewBuiltin("computer_fetch", computerGet(credName, logger)),
		"computer_send_mdm_command": starlark.NewBuiltin("computer_send_mdm_command", computerSendMDMCommand(credName, logger)),
		"computer_reboot":           starlark.NewBuiltin("computer_reboot", computerReboot(credName, logger)),
		"computer_redeploy":         starlark.NewBuiltin("computer_redeploy", computerRedeploy(credName, logger)),
		"computer_shutdown":         starlark.NewBuiltin("computer_shutdown", computerShutdown(credName, logger)),
		"computer_lock":             starlark.NewBuiltin("computer_lock", computerLock(credName, logger)),
		"computer_logout":           starlark.NewBuiltin("computer_logout", computerLogout(credName, logger)),
		"computer_erase":            starlark.NewBuiltin("computer_erase", computerErase(credName, logger)),
		"computer_device_info":      starlark.NewBuiltin("computer_device_info", computerDeviceInfo(credName, logger)),
		"computer_device_location":  starlark.NewBuiltin("computer_device_location", computerDeviceLocation(credName, logger)),
		"computer_security_info":    starlark.NewBuiltin("computer_security_info", computerSecurityInfo(credName, logger)),
		"computer_installed_apps":   starlark.NewBuiltin("computer_installed_apps", computerInstalledApps(credName, logger)),
		"computer_managed_apps":     starlark.NewBuiltin("computer_managed_apps", computerManagedApps(credName, logger)),
		"computer_certificates":     starlark.NewBuiltin("computer_certificates", computerCertificates(credName, logger)),

		// MDM: general command query (distinct from computer-specific senders).
		"mdm_command_find":   starlark.NewBuiltin("mdm_command_find", mdmCommandFind(credName, logger)),
		"mdm_fetch_commands": starlark.NewBuiltin("mdm_fetch_commands", mdmFetchCommands(credName, logger)),
	}
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
				fmt.Fprintf(os.Stderr, "[trace] %s:%d  → %s(%s)\n", pos.Filename(), pos.Line, bName, formatTraceArgs(args, kwargs))
				result, err := starlark.Call(t, orig, args, kwargs)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[trace] %s:%d  ← %s error: %v\n", pos.Filename(), pos.Line, bName, err)
				} else {
					fmt.Fprintf(os.Stderr, "[trace] %s:%d  ← %s = %s\n", pos.Filename(), pos.Line, bName, result)
				}
				return result, err
			})
		}
	}
	return dict
}

func policyGetID(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var trigger starlark.String
		forceRefresh := starlark.Bool(false)
		if err := starlark.UnpackArgs("policy_get_id", args, kwargs, "trigger", &trigger, "force_refresh?", &forceRefresh); err != nil {
			return nil, err
		}
		cred := getCred(thread, credName)
		id, err := buddy.PolicyFind(cred, scriptOpts, string(trigger), bool(forceRefresh), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.MakeInt(id)), nil
	}
}

func policyGetEnable(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		if err := starlark.UnpackArgs("policy_get_enable", args, kwargs, "policy_id", &policyID); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		cred := getCred(thread, credName)
		state, err := buddy.PolicyGetEnableValue(cred, int(pid), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		normalized := state
		// Normalize legacy values to match builtin docs ("true"/"false").
		if state == "enabled" {
			normalized = "true"
		} else if state == "disabled" {
			normalized = "false"
		}

		return resultOK(starlark.String(normalized)), nil
	}
}

func policyGetDetails(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var cred starlark.String
		if err := starlark.UnpackArgs("policy_get_details", args, kwargs, "policy_id", &policyID, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		d, err := buddy.PolicyGetDetails(getCred(thread, c), int(pid), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		// Build starlark dict from PolicyDetails
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
		return resultOK(out), nil
	}
}

func policySetSS(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var cred starlark.String
		var policyID starlark.Int
		var version starlark.String
		var groupIDs *starlark.List
		var displayName, description, installText, reinstallText starlark.String
		var forceView, feature, selfService *starlark.Bool
		if err := starlark.UnpackArgs("policy_set_ss", args, kwargs,
			"policy_id", &policyID,
			"version?", &version, "group_ids?", &groupIDs,
			"display_name?", &displayName, "description?", &description,
			"install_text?", &installText, "reinstall_text?", &reinstallText,
			"force_view?", &forceView, "feature?", &feature, "self_service?", &selfService,
			"cred?", &cred,
		); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
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
		err := buddy.PolicySSUpdate(getCred(thread, c), scriptOpts, int(pid), ssOpts, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policySetEnable(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var cred starlark.String
		if err := starlark.UnpackArgs("policy_set_enable", args, kwargs, "policy_id", &policyID, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.PolicyEnable(getCred(thread, c), scriptOpts, int(pid), true, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policySetDisable(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var cred starlark.String
		if err := starlark.UnpackArgs("policy_set_disable", args, kwargs, "policy_id", &policyID, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.PolicyEnable(getCred(thread, c), scriptOpts, int(pid), false, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policySetTrigger(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var trigger, cred starlark.String
		if err := starlark.UnpackArgs("policy_set_trigger", args, kwargs, "policy_id", &policyID, "trigger", &trigger, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.PolicyTrigger(getCred(thread, c), scriptOpts, int(pid), string(trigger), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policySetName(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var newName, cred starlark.String
		if err := starlark.UnpackArgs("policy_set_name", args, kwargs, "policy_id", &policyID, "new_name", &newName, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.PolicySetName(getCred(thread, c), scriptOpts, int(pid), string(newName), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policySetFrequency(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var frequency, cred starlark.String
		if err := starlark.UnpackArgs("policy_set_frequency", args, kwargs, "policy_id", &policyID, "frequency", &frequency, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.PolicySetFrequency(getCred(thread, c), scriptOpts, int(pid), string(frequency), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policySetInstall(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var version, cred starlark.String
		if err := starlark.UnpackArgs("policy_set_install", args, kwargs, "policy_id", &policyID, "version", &version, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		paramIndices := []int{4, 5, 6}
		err := buddy.PolicyInstall(getCred(thread, c), scriptOpts, int(pid), string(version), paramIndices, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policyClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var version, cred starlark.String
		if err := starlark.UnpackArgs("policy_clone", args, kwargs, "policy_id", &policyID, "version", &version, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		paramIndices := []int{4, 5, 6}
		err := buddy.PolicyClone(getCred(thread, c), scriptOpts, int(pid), "", "", string(version), "", paramIndices, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policyCreateSS(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var trigger, displayName, category, cred starlark.String
		var scopeGroupID starlark.Int
		if err := starlark.UnpackArgs("policy_create_ss", args, kwargs,
			"trigger", &trigger, "display_name", &displayName,
			"category?", &category,
			"scope_group_id?", &scopeGroupID,
			"cred?", &cred,
		); err != nil {
			return nil, err
		}
		sgid, _ := scopeGroupID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		// For scripts/tests, default to no script attachment unless explicitly requested.
		policyID, err := buddy.PolicyCreateSS(getCred(thread, c), scriptOpts, string(trigger), string(displayName), string(category), int(sgid), 0, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.MakeInt(policyID)), nil
	}
}

func policySetPackage(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var app, version, trigger, packagePattern, cred starlark.String
		var policyID starlark.Int = starlark.MakeInt(0)
		if err := starlark.UnpackArgs("policy_set_package", args, kwargs, "app", &app, "version", &version, "policy_id?", &policyID, "trigger?", &trigger, "package_pattern?", &packagePattern, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		triggerOverride := ""
		if trigger != "" {
			triggerOverride = string(trigger)
		}
		packageOverride := ""
		if packagePattern != "" {
			packageOverride = string(packagePattern)
		}
		err := buddy.PolicyPackage(getCred(thread, c), scriptOpts, strings.ToLower(string(app)), string(version), int(pid), triggerOverride, packageOverride, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// group_list_smart(filter?, cred?) → {ok, value: [{id, name, description}, ...]}
func groupListSmart(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return groupListBuiltin("group_list_smart", buddy.GroupSearchSmart, credName, logger)
}

// group_list_static(filter?, cred?) → {ok, value: [{id, name, description}, ...]}
func groupListStatic(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return groupListBuiltin("group_list_static", buddy.GroupSearchStatic, credName, logger)
}

type groupSearchFunc func(credName, filter string, logger *slog.Logger) ([]buddy.GroupSummary, error)

func groupListBuiltin(name string, search groupSearchFunc, credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter, cred starlark.String
		if err := starlark.UnpackArgs(name, args, kwargs, "filter?", &filter, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		records, err := search(c, string(filter), logger)
		if err != nil {
			return resultErr(err.Error()), nil
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

// group_find_id(name, cred?) → {ok, value: {id, name}}
func groupFindID(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, cred starlark.String
		if err := starlark.UnpackArgs("group_find_id", args, kwargs, "name", &name, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		id, groupName, err := buddy.GroupFindID(c, string(name), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if id == "" {
			return resultErr(fmt.Sprintf("no group found for name %q", string(name))), nil
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(id))
		_ = d.SetKey(starlark.String("name"), starlark.String(groupName))
		return resultOK(d), nil
	}
}

func groupGetVersion(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		if err := starlark.UnpackArgs("group_get_version", args, kwargs, "group_id", &groupID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		version, err := buddy.GroupGetVersionValue(getCred(thread, c), string(groupID), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(version)), nil
	}
}

func groupSetVersion(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, version, cred starlark.String
		if err := starlark.UnpackArgs("group_set_version", args, kwargs, "group_id", &groupID, "version", &version, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.GroupVersion(getCred(thread, c), scriptOpts, string(groupID), string(version), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// static_group_list_computers(group_id, filter?, cred?) → {ok, value: list[dict]}
func staticGroupListComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, filter, cred starlark.String
		if err := starlark.UnpackArgs("static_group_list_computers", args, kwargs, "group_id", &groupID, "filter?", &filter, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		members, err := buddy.StaticGroupFetchMembers(getCred(thread, c), string(groupID))
		if err != nil {
			return resultErr(err.Error()), nil
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

// static_group_add_computers(group_id, computer_ids, cred?) → {ok, value: None}
func staticGroupAddComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("static_group_add_computers", args, kwargs, "group_id", &groupID, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, err := parseComputerIDStringsFromList(computerIDs, "static_group_add_computers")
		if err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		_, err = buddy.StaticGroupAddComputers(getCred(thread, c), scriptOpts, string(groupID), ids, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// static_group_remove_computers(group_id, computer_ids, cred?) → {ok, value: None}
func staticGroupRemoveComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("static_group_remove_computers", args, kwargs, "group_id", &groupID, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, err := parseComputerIDStringsFromList(computerIDs, "static_group_remove_computers")
		if err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		_, err = buddy.StaticGroupRemoveComputers(getCred(thread, c), scriptOpts, string(groupID), ids, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// static_group_set_computers(group_id, computer_ids, cred?) → {ok, value: None}
func staticGroupSetComputers(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("static_group_set_computers", args, kwargs, "group_id", &groupID, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, err := parseComputerIDStringsFromList(computerIDs, "static_group_set_computers")
		if err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		_, err = buddy.StaticGroupSetComputers(getCred(thread, c), scriptOpts, string(groupID), ids, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// smart_group_fetch(group_id, cred?) → {ok, value: dict}
func smartGroupFetch(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		if err := starlark.UnpackArgs("smart_group_fetch", args, kwargs, "group_id", &groupID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		body, err := buddy.GroupFetchSmartRaw(getCred(thread, c), string(groupID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		// Decode JSON into a Starlark dict/list tree.
		val, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err)), nil
		}
		return resultOK(val), nil
	}
}

// smart_group_put(group_id, body, cred?) → {ok, value: None}
func smartGroupPut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		var body starlark.Value
		if err := starlark.UnpackArgs("smart_group_put", args, kwargs, "group_id", &groupID, "body", &body, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		// For scripts, always skip confirm.
		opts := scriptOpts
		opts.SkipConfirm = true
		if err := buddy.GroupPutSmartRaw(getCred(thread, c), opts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// smart_group_get_criteria(group_id, cred?) → {ok, value: list[dict]}
func smartGroupGetCriteria(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		if err := starlark.UnpackArgs("smart_group_get_criteria", args, kwargs, "group_id", &groupID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		body, err := buddy.GroupFetchSmartRaw(getCred(thread, c), string(groupID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err)), nil
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict"), nil
		}
		critVal, found, err := d.Get(starlark.String("criteria"))
		if err != nil {
			return resultErr(fmt.Sprintf("lookup criteria: %v", err)), nil
		}
		if !found {
			// No criteria field: return empty list.
			return resultOK(starlark.NewList(nil)), nil
		}
		if _, ok := critVal.(*starlark.List); !ok {
			return resultErr("smart group criteria is not a list"), nil
		}
		return resultOK(critVal), nil
	}
}

// smart_group_set_criteria(group_id, criteria, cred?) → {ok, value: None}
func smartGroupSetCriteria(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		var criteria *starlark.List
		if err := starlark.UnpackArgs("smart_group_set_criteria", args, kwargs, "group_id", &groupID, "criteria", &criteria, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		body, err := buddy.GroupFetchSmartRaw(getCred(thread, c), string(groupID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err)), nil
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict"), nil
		}
		if err := d.SetKey(starlark.String("criteria"), criteria); err != nil {
			return resultErr(fmt.Sprintf("set criteria: %v", err)), nil
		}
		jsonStr, err := StarlarkToJSON(d)
		if err != nil {
			return resultErr(fmt.Sprintf("encode updated smart group JSON: %v", err)), nil
		}
		opts := scriptOpts
		opts.SkipConfirm = true
		if err := buddy.GroupPutSmartRaw(getCred(thread, c), opts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// smart_group_find_criteria(group_id, name_substring, cred?) → {ok, value: list[dict]}
func smartGroupFindCriteria(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, nameSubstr, cred starlark.String
		if err := starlark.UnpackArgs("smart_group_find_criteria", args, kwargs, "group_id", &groupID, "name_substring", &nameSubstr, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		body, err := buddy.GroupFetchSmartRaw(getCred(thread, c), string(groupID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err)), nil
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict"), nil
		}
		critVal, found, err := d.Get(starlark.String("criteria"))
		if err != nil {
			return resultErr(fmt.Sprintf("lookup criteria: %v", err)), nil
		}
		if !found {
			return resultOK(starlark.NewList(nil)), nil
		}
		lst, ok := critVal.(*starlark.List)
		if !ok {
			return resultErr("smart group criteria is not a list"), nil
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

// smart_group_update_criterion_value(group_id, name_substring, new_value, cred?) → {ok, value: None}
func smartGroupUpdateCriterionValue(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, nameSubstr, newValue, cred starlark.String
		if err := starlark.UnpackArgs("smart_group_update_criterion_value", args, kwargs,
			"group_id", &groupID, "name_substring", &nameSubstr, "new_value", &newValue, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		body, err := buddy.GroupFetchSmartRaw(getCred(thread, c), string(groupID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		root, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err)), nil
		}
		d, ok := root.(*starlark.Dict)
		if !ok {
			return resultErr("smart group JSON root is not a dict"), nil
		}
		critVal, found, err := d.Get(starlark.String("criteria"))
		if err != nil {
			return resultErr(fmt.Sprintf("lookup criteria: %v", err)), nil
		}
		if !found {
			return resultErr("smart group has no criteria array"), nil
		}
		lst, ok := critVal.(*starlark.List)
		if !ok {
			return resultErr("smart group criteria is not a list"), nil
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
					return resultErr(fmt.Sprintf("set criterion value: %v", err)), nil
				}
				changed = true
			}
		}
		if !changed {
			return resultErr(fmt.Sprintf("no criteria found with name containing %q", string(nameSubstr))), nil
		}
		jsonStr, err := StarlarkToJSON(d)
		if err != nil {
			return resultErr(fmt.Sprintf("encode updated smart group JSON: %v", err)), nil
		}
		opts := scriptOpts
		opts.SkipConfirm = true
		if err := buddy.GroupPutSmartRaw(getCred(thread, c), opts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func rolePrivilegesList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var cred starlark.String
		if err := starlark.UnpackArgs("role_privileges_list", args, kwargs, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		privs, err := buddy.RolePrivilegesList(getCred(thread, c), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		elems := make([]starlark.Value, len(privs))
		for i, p := range privs {
			elems[i] = starlark.String(p)
		}
		return resultOK(starlark.NewList(elems)), nil
	}
}

func roleList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var cred starlark.String
		if err := starlark.UnpackArgs("role_list", args, kwargs, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		roles, err := buddy.RoleListValues(getCred(thread, c), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := starlark.NewList(nil)
		for _, r := range roles {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(r.ID))
			_ = d.SetKey(starlark.String("display_name"), starlark.String(r.DisplayName))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// role_fetch_list(cred?) → {ok, value: list of dict}
func roleFetchList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var cred starlark.String
		if err := starlark.UnpackArgs("role_fetch_list", args, kwargs, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		roles, err := buddy.RoleListValues(getCred(thread, c), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(roles)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// role_fetch(role_id, cred?) → {ok, value: dict}
func roleFetch(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var roleID starlark.String
		var cred starlark.String
		if err := starlark.UnpackArgs("role_fetch", args, kwargs, "role_id", &roleID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		role, err := buddy.RoleFetch(getCred(thread, c), string(roleID), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(role)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// role_put(role_id, dict, cred?) → {ok, value: dict}
func rolePut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var roleID starlark.String
		var bodyVal starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("role_put", args, kwargs, "role_id", &roleID, "dict", &bodyVal, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict"), nil
		}
		// Resolve desired fields, preserving existing where unspecified.
		existing, err := buddy.RoleFetch(getCred(thread, c), string(roleID), logger)
		if err != nil {
			return resultErr(err.Error()), nil
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
		updated, err := buddy.RoleUpdateWithResult(getCred(thread, c), string(roleID), scriptOpts, displayName, privs, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if updated == nil {
			return resultErr("role_put: cancelled"), nil
		}
		b, err := json.Marshal(updated)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

func roleCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Backward compatible signature:
		//   role_create(cred, display_name, privileges)
		// Component signature:
		//   role_create(dict, cred?)
		if len(args) > 0 {
			if d, ok := args[0].(*starlark.Dict); ok {
				cred := starlark.String("")
				_ = starlark.UnpackArgs("role_create", starlark.Tuple{}, kwargs, "cred?", &cred)
				c := getCred(thread, credName)
				if cred != "" {
					c = getCred(thread, string(cred))
				}
				g, err := StarlarkValueToGo(d)
				if err != nil {
					return resultErr(err.Error()), nil
				}
				m, ok := g.(map[string]interface{})
				if !ok {
					return resultErr("role_create: dict must be a map"), nil
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
					return resultErr("role_create: missing displayName"), nil
				}
				role, err := buddy.RoleCreateWithID(c, scriptOpts, displayName, privs, logger)
				if err != nil {
					return resultErr(err.Error()), nil
				}
				if role == nil {
					return resultErr("role_create: cancelled"), nil
				}
				return resultOK(starlark.String(role.ID)), nil
			}
		}

		var displayName, cred starlark.String
		var privileges *starlark.List
		if err := starlark.UnpackArgs("role_create", args, kwargs, "display_name", &displayName, "privileges", &privileges, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		var privs []string
		for i := 0; i < privileges.Len(); i++ {
			v := privileges.Index(i)
			if s, ok := v.(starlark.String); ok {
				privs = append(privs, string(s))
			} else {
				return nil, fmt.Errorf("role_create: privileges must be a list of strings")
			}
		}
		role, err := buddy.RoleCreateWithID(getCred(thread, c), scriptOpts, string(displayName), privs, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if role == nil {
			return resultErr("role_create: cancelled"), nil
		}
		return resultOK(starlark.String(role.ID)), nil
	}
}

// client_fetch_list(cred?) → {ok, value: list of dict}
func clientFetchList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var cred starlark.String
		if err := starlark.UnpackArgs("client_fetch_list", args, kwargs, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		clients, err := buddy.ClientListValues(getCred(thread, c), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(clients)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// client_fetch(client_id, cred?) → {ok, value: dict}
func clientFetch(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var clientID starlark.String
		var cred starlark.String
		if err := starlark.UnpackArgs("client_fetch", args, kwargs, "client_id", &clientID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		client, err := buddy.ClientFetch(getCred(thread, c), string(clientID), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(client)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// client_put(client_id, dict, cred?) → {ok, value: dict}
func clientPut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var clientID starlark.String
		var bodyVal starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("client_put", args, kwargs, "client_id", &clientID, "dict", &bodyVal, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict"), nil
		}
		updated, err := buddy.ClientPutFields(getCred(thread, c), scriptOpts, string(clientID), m, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if updated == nil {
			return resultErr("client_put: cancelled"), nil
		}
		b, err := json.Marshal(updated)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

func clientCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Backward compatible signature:
		//   client_create(cred, display_name, scopes)
		// Component signature:
		//   client_create(dict, cred?)
		if len(args) > 0 {
			if d, ok := args[0].(*starlark.Dict); ok {
				cred := starlark.String("")
				_ = starlark.UnpackArgs("client_create", starlark.Tuple{}, kwargs, "cred?", &cred)
				c := getCred(thread, credName)
				if cred != "" {
					c = getCred(thread, string(cred))
				}
				g, err := StarlarkValueToGo(d)
				if err != nil {
					return resultErr(err.Error()), nil
				}
				m, ok := g.(map[string]interface{})
				if !ok {
					return resultErr("client_create: dict must be a map"), nil
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
					return resultErr("client_create: missing displayName"), nil
				}
				client, err := buddy.ClientCreateWithResult(c, scriptOpts, displayName, scopeList, logger)
				if err != nil {
					return resultErr(err.Error()), nil
				}
				if client == nil {
					return resultErr("client_create: cancelled"), nil
				}
				out := new(starlark.Dict)
				_ = out.SetKey(starlark.String("id"), starlark.String(strconv.Itoa(client.ID)))
				_ = out.SetKey(starlark.String("client_id"), starlark.String(client.ClientID))
				return resultOK(out), nil
			}
		}

		var displayName, cred starlark.String
		var scopes *starlark.List
		if err := starlark.UnpackArgs("client_create", args, kwargs, "display_name", &displayName, "scopes", &scopes, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
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
		client, err := buddy.ClientCreateWithResult(getCred(thread, c), scriptOpts, string(displayName), scopeList, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if client == nil {
			return resultErr("client_create: cancelled"), nil
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
		var cred starlark.String
		if err := starlark.UnpackArgs("policy_set_scope_all", args, kwargs, "policy_id", &policyID, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.PolicyScopeAll(getCred(thread, c), scriptOpts, int(pid), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policyDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("policy_delete", args, kwargs, "policy_ids", &policyIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		var ids []int
		for i := 0; i < policyIDs.Len(); i++ {
			n, ok := policyIDs.Index(i).(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("policy_delete: policy_ids must be a list of ints")
			}
			v, _ := n.Int64()
			ids = append(ids, int(v))
		}
		err := buddy.PolicyDelete(getCred(thread, c), scriptOpts, ids, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func policyFindIDs(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var keywords *starlark.List
		var excludeIDs *starlark.List
		var cred starlark.String
		var forceRefresh starlark.Bool
		if err := starlark.UnpackArgs("policy_find_ids", args, kwargs,
			"keywords", &keywords, "exclude_ids?", &excludeIDs, "force_refresh?", &forceRefresh, "cred?", &cred,
		); err != nil {
			return nil, err
		}
		var kw []string
		for i := 0; i < keywords.Len(); i++ {
			v := keywords.Index(i)
			if s, ok := v.(starlark.String); ok {
				kw = append(kw, string(s))
			} else {
				return nil, fmt.Errorf("policy_find_ids: keywords must be a list of strings")
			}
		}
		var exclude []int
		if excludeIDs != nil {
			for i := 0; i < excludeIDs.Len(); i++ {
				n, ok := excludeIDs.Index(i).(starlark.Int)
				if !ok {
					return nil, fmt.Errorf("policy_find_ids: exclude_ids must be a list of ints")
				}
				v, _ := n.Int64()
				exclude = append(exclude, int(v))
			}
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		ids, err := buddy.PolicyFindByKeywords(getCred(thread, c), scriptOpts, kw, exclude, bool(forceRefresh), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := starlark.NewList(nil)
		for _, id := range ids {
			_ = list.Append(starlark.MakeInt(id))
		}
		return resultOK(list), nil
	}
}

// policy_fetch_xml(policy_id, cred?) → {ok, value: str}
func policyFetchXML(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var cred starlark.String
		if err := starlark.UnpackArgs("policy_fetch_xml", args, kwargs, "policy_id", &policyID, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		xmlBytes, err := buddy.PolicyFetchXML(getCred(thread, c), int(pid))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(string(xmlBytes))), nil
	}
}

// policy_put_xml(policy_id, xml_str, cred?)
func policyPutXML(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var xmlStr starlark.String
		var cred starlark.String
		if err := starlark.UnpackArgs("policy_put_xml", args, kwargs, "policy_id", &policyID, "xml_str", &xmlStr, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := policyID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.PolicyPutXML(getCred(thread, c), scriptOpts, int(pid), []byte(string(xmlStr)), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// group_fetch(group_id, cred?) → {ok, value: dict}
func groupFetch(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var cred starlark.String
		if err := starlark.UnpackArgs("group_fetch", args, kwargs, "group_id", &groupID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		body, err := buddy.GroupFetchSmartRaw(getCred(thread, c), string(groupID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// group_put(group_id, dict, cred?) → {ok, value: None}
func groupPut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var bodyVal starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("group_put", args, kwargs, "group_id", &groupID, "dict", &bodyVal, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		jsonStr, err := StarlarkToJSON(bodyVal)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		// If group_id is "-1", create via POST; otherwise update via PUT.
		if string(groupID) == "-1" {
			// For scripts, always skip confirm.
			opts := scriptOpts
			opts.SkipConfirm = true
			if _, err := buddy.GroupCreateSmartRaw(getCred(thread, c), opts, []byte(jsonStr), logger); err != nil {
				return resultErr(err.Error()), nil
			}
		} else {
			if err := buddy.GroupPutSmartRaw(getCred(thread, c), scriptOpts, string(groupID), []byte(jsonStr), logger); err != nil {
				return resultErr(err.Error()), nil
			}
		}
		return resultOK(starlark.None), nil
	}
}

// ---------------------------------------------------------------------------
// Package builtins
// ---------------------------------------------------------------------------

// package_list(filter="") → list of {id, packageName, fileName} dicts
func packageList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		if err := starlark.UnpackArgs("package_list", args, kwargs, "filter?", &filter); err != nil {
			return nil, err
		}
		cred := getCred(thread, credName)
		results, err := buddy.PackageSearch(cred, string(filter), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := starlark.NewList(nil)
		for _, p := range results {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(p.ID))
			_ = d.SetKey(starlark.String("packageName"), starlark.String(p.PackageName))
			_ = d.SetKey(starlark.String("fileName"), starlark.String(p.FileName))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// package_fetch(package_id, cred?) → {ok, value: dict}
func packageFetch(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var cred starlark.String
		if err := starlark.UnpackArgs("package_fetch", args, kwargs, "package_id", &id, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		pkg, err := buddy.PackageFetchFull(getCred(thread, c), string(id))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, mErr := json.Marshal(pkg)
		if mErr != nil {
			return resultErr(mErr.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// package_put(package_id, dict, cred?) → {ok, value: dict}
func packagePut(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var bodyVal starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("package_put", args, kwargs, "package_id", &id, "dict", &bodyVal, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict"), nil
		}
		updated, err := buddy.PackagePutFields(getCred(thread, c), scriptOpts, string(id), m, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, mErr := json.Marshal(updated)
		if mErr != nil {
			return resultErr(mErr.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// package_find_id(app, version, pattern?) → {ok, value: {id, name}}
func packageFindID(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var app, version, pattern starlark.String
		if err := starlark.UnpackArgs("package_find_id", args, kwargs, "app", &app, "version", &version, "pattern?", &pattern); err != nil {
			return nil, err
		}
		cred := getCred(thread, credName)
		patternOverride := ""
		if pattern != "" {
			patternOverride = string(pattern)
		}
		id, name, err := buddy.PackageFindID(cred, string(app), string(version), patternOverride, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if id == "" {
			return resultErr(fmt.Sprintf("no package found for app %q version %q", string(app), string(version))), nil
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(id))
		_ = d.SetKey(starlark.String("name"), starlark.String(name))
		return resultOK(d), nil
	}
}

// package_delete(ids=[...]) → {ok, value: None}
func packageDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var ids *starlark.List
		if err := starlark.UnpackArgs("package_delete", args, kwargs, "ids", &ids); err != nil {
			return nil, err
		}
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
		cred := getCred(thread, credName)
		if err := buddy.PackageDelete(cred, scriptOpts, idStrs, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func packageUpload(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var file, name, categoryID, cred starlark.String
		var priority starlark.Int
		if err := starlark.UnpackArgs("package_upload", args, kwargs,
			"file", &file,
			"name?", &name, "category_id?", &categoryID, "priority?", &priority, "cred?", &cred,
		); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		catID := "-1"
		if categoryID != "" {
			catID = string(categoryID)
		}
		prio := 10
		if priority != starlark.MakeInt(0) {
			p64, _ := priority.Int64()
			prio = int(p64)
		}
		err := buddy.PackageUpload(getCred(thread, c), scriptOpts, string(file), string(name), catID, prio, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// script_list(filter?, cred?) → {ok, value: [{id, name, categoryId, categoryName}, ...]}
func scriptList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter, cred starlark.String
		if err := starlark.UnpackArgs("script_list", args, kwargs, "filter?", &filter, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		credResolved := getCred(thread, c)
		results, err := buddy.ScriptSearch(credResolved, string(filter), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := starlark.NewList(nil)
		for _, s := range results {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(s.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(s.Name))
			_ = d.SetKey(starlark.String("categoryId"), starlark.String(s.CategoryID))
			_ = d.SetKey(starlark.String("categoryName"), starlark.String(s.CategoryName))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// script_get_id(name) → {ok, value: id_string}
func scriptGetID(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name starlark.String
		if err := starlark.UnpackArgs("script_get_id", args, kwargs, "name", &name); err != nil {
			return nil, err
		}
		cred := getCred(thread, credName)
		id, err := buddy.ScriptGetID(cred, string(name), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(id)), nil
	}
}

// script_get_contents(script_id, cred?) → {ok, value: contents_string}
func scriptGetContents(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID, cred starlark.String
		if err := starlark.UnpackArgs("script_get_contents", args, kwargs, "script_id", &scriptID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		contents, err := buddy.ScriptGetContentsStr(getCred(thread, c), string(scriptID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(contents)), nil
	}
}

// script_update(script_id, file, cred?) → {ok, value: None}
func scriptUpdate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID, file, cred starlark.String
		if err := starlark.UnpackArgs("script_update", args, kwargs, "script_id", &scriptID, "file", &file, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		if err := buddy.ScriptUpdate(getCred(thread, c), scriptOpts, string(scriptID), string(file), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func groupDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		if err := starlark.UnpackArgs("group_delete", args, kwargs, "group_id", &groupID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.GroupDelete(getCred(thread, c), scriptOpts, string(groupID), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// error is like print but writes to stderr (same *args and optional sep= as print).
func errorBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
	fmt.Fprintln(os.Stderr, buf.String())
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

func exitBuiltin(logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var code starlark.Int = starlark.MakeInt(1)
		if err := starlark.UnpackArgs("exit", args, kwargs, "code?", &code); err != nil {
			return nil, err
		}
		c, _ := code.Int64()
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

func getCred(thread *starlark.Thread, defaultCred string) string {
	if thread.Local("cred") != nil {
		if s, ok := thread.Local("cred").(string); ok {
			return s
		}
	}
	return defaultCred
}

// json_decode(json_str) → {ok, value: dict or list} or {ok: False, error: str}
func jsonDecodeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var jsonStr starlark.String
	if err := starlark.UnpackArgs("json_decode", args, kwargs, "json_str", &jsonStr); err != nil {
		return nil, err
	}
	val, err := JSONDecodeToStarlark(string(jsonStr))
	if err != nil {
		return resultErr(err.Error()), nil
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
		return resultErr(err.Error()), nil
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
		return resultErr(err.Error()), nil
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
		return resultErr(err.Error()), nil
	}
	return resultOK(starlark.String(s)), nil
}

// file_read(path) → {ok, value: str} — UTF-8 text only; use file_read_bytes for binary.
func fileReadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	if err := starlark.UnpackArgs("file_read", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(string(path))
	if err != nil {
		return resultErr(err.Error()), nil
	}
	if !utf8.Valid(b) {
		return resultErr("file is not valid UTF-8; use file_read_bytes for binary data"), nil
	}
	return resultOK(starlark.String(string(b))), nil
}

// file_read_bytes(path) → {ok, value: bytes}
func fileReadBytesBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	if err := starlark.UnpackArgs("file_read_bytes", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(string(path))
	if err != nil {
		return resultErr(err.Error()), nil
	}
	return resultOK(starlark.Bytes(b)), nil
}

// file_write(path, data) → {ok, value: None} — data is str (UTF-8) or bytes.
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
		return resultErr("data must be str or bytes"), nil
	}
	if err := os.WriteFile(string(path), raw, 0o644); err != nil {
		return resultErr(err.Error()), nil
	}
	return resultOK(starlark.None), nil
}

// ---------------------------------------------------------------------------
// Declarative workflow: step constructor builtins
// Each returns a Starlark dict describing the step; no side effects.
// ---------------------------------------------------------------------------

func stepSetPackageBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var app, version, fromTrigger, packagePattern starlark.String
	if err := starlark.UnpackArgs("step_set_package", args, kwargs,
		"app", &app, "version", &version, "from_trigger", &fromTrigger, "package_pattern?", &packagePattern); err != nil {
		return nil, err
	}
	d := stepDict(stepTypeSetPackage,
		"app", string(app),
		"version", string(version),
		"from_trigger", string(fromTrigger),
	)
	if string(packagePattern) != "" {
		_ = d.SetKey(starlark.String("package_pattern"), packagePattern)
	}
	return d, nil
}

func stepClonePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fromTrigger, version, app, oldVersion, packagePattern starlark.String
	var params *starlark.List
	if err := starlark.UnpackArgs("step_clone_policy", args, kwargs,
		"from_trigger", &fromTrigger, "version", &version,
		"app?", &app, "old_version?", &oldVersion, "package_pattern?", &packagePattern, "params?", &params); err != nil {
		return nil, err
	}
	d := stepDict(stepTypeClonePolicy,
		"from_trigger", string(fromTrigger),
		"version", string(version),
	)
	if string(app) != "" {
		_ = d.SetKey(starlark.String("app"), app)
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
	return d, nil
}

func stepSetSSBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var version starlark.String
	var policyID starlark.Int
	var groupIDs *starlark.List
	if err := starlark.UnpackArgs("step_set_ss", args, kwargs,
		"policy_id", &policyID, "version", &version, "group_ids?", &groupIDs); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	d := stepDict(stepTypeSetSS,
		"policy_id", int(pid),
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
		"group_id", &groupID, "version", &version); err != nil {
		return nil, err
	}
	return stepDict(stepTypeSetGroupVersion,
		"group_id", string(groupID),
		"version", string(version),
	), nil
}

func stepSetInstallBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var version starlark.String
	var policyID starlark.Int
	var params *starlark.List
	if err := starlark.UnpackArgs("step_set_install", args, kwargs,
		"policy_id", &policyID, "version", &version, "params?", &params); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	d := stepDict(stepTypeSetInstall,
		"policy_id", int(pid),
		"version", string(version),
	)
	if params != nil {
		_ = d.SetKey(starlark.String("params"), params)
	}
	return d, nil
}

func stepEnablePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_enable_policy", args, kwargs, "policy_id", &policyID); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeEnablePolicy, "policy_id", int(pid)), nil
}

func stepDisablePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_disable_policy", args, kwargs, "policy_id", &policyID); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeDisablePolicy, "policy_id", int(pid)), nil
}

func stepSetTriggerBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var trigger starlark.String
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_set_trigger", args, kwargs,
		"policy_id", &policyID, "trigger", &trigger); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeSetTrigger,
		"policy_id", int(pid),
		"trigger", string(trigger),
	), nil
}

func stepSetScopeBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	var groupIDs *starlark.List
	if err := starlark.UnpackArgs("step_set_scope", args, kwargs,
		"policy_id", &policyID, "group_ids", &groupIDs); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	d := stepDict(stepTypeSetScope, "policy_id", int(pid))
	_ = d.SetKey(starlark.String("group_ids"), groupIDs)
	return d, nil
}

func stepSetScopeAllBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyID starlark.Int
	if err := starlark.UnpackArgs("step_set_scope_all", args, kwargs, "policy_id", &policyID); err != nil {
		return nil, err
	}
	pid, _ := policyID.Int64()
	return stepDict(stepTypeSetScopeAll, "policy_id", int(pid)), nil
}

func stepDeletePolicyBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var policyIDs *starlark.List
	if err := starlark.UnpackArgs("step_delete_policy", args, kwargs, "policy_ids", &policyIDs); err != nil {
		return nil, err
	}
	d := stepDict(stepTypeDeletePolicy)
	_ = d.SetKey(starlark.String("policy_ids"), policyIDs)
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

// profile_list(filter="") → list of dicts {id, name}
func profileList(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		filter := starlark.String("")
		if err := starlark.UnpackArgs("profile_list", args, kwargs, "filter?", &filter); err != nil {
			return nil, err
		}
		cred := getCred(thread, credName)
		records, err := buddy.ProfileSearch(cred, string(filter), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := new(starlark.List)
		for _, r := range records {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.MakeInt(r.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// profile_find_id(name) → {ok, value: {id, name}}
func profileFindID(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name starlark.String
		if err := starlark.UnpackArgs("profile_find_id", args, kwargs, "name", &name); err != nil {
			return nil, err
		}
		cred := getCred(thread, credName)
		id, profileName, err := buddy.ProfileFindID(cred, string(name), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if id == 0 {
			return resultErr(fmt.Sprintf("no profile found for name %q", string(name))), nil
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.MakeInt(id))
		_ = d.SetKey(starlark.String("name"), starlark.String(profileName))
		return resultOK(d), nil
	}
}

// profile_get_payloads(profile_id) → {ok, value: str} (payloads plist XML)
func profileGetPayloads(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		if err := starlark.UnpackArgs("profile_get_payloads", args, kwargs, "profile_id", &profileID); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		cred := getCred(thread, credName)
		var buf bytes.Buffer
		if err := buddy.ProfileGetPayloads(cred, int(pid), &buf, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(buf.String())), nil
	}
}

// profile_get_payload(profile_id, domain) → {ok, value: str} (single payload plist XML)
func profileGetPayload(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var domain starlark.String
		if err := starlark.UnpackArgs("profile_get_payload", args, kwargs, "profile_id", &profileID, "domain", &domain); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		cred := getCred(thread, credName)
		var buf bytes.Buffer
		if err := buddy.ProfileGetPayloadByDomain(cred, int(pid), string(domain), &buf, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(buf.String())), nil
	}
}

// profile_list_payloads(profile_id) → {ok, value: list of str} (PayloadIdentifiers)
func profileListPayloads(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		if err := starlark.UnpackArgs("profile_list_payloads", args, kwargs, "profile_id", &profileID); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		cred := getCred(thread, credName)
		domains, err := buddy.ProfileListPayloads(cred, int(pid), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := new(starlark.List)
		for _, d := range domains {
			_ = list.Append(starlark.String(d))
		}
		return resultOK(list), nil
	}
}

// profile_redeploy(profile_id) → {ok, value: None}
func profileRedeploy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		if err := starlark.UnpackArgs("profile_redeploy", args, kwargs, "profile_id", &profileID); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		cred := getCred(thread, credName)
		err := buddy.ProfileRedeploy(cred, scriptOpts, int(pid), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func profileSetPayload(logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var domain, dictFile starlark.String
		redeploy := starlark.Bool(false)
		if err := starlark.UnpackArgs("profile_set_payload", args, kwargs,
			"profile_id", &profileID,
			"domain", &domain,
			"dict_file", &dictFile,
			"redeploy?", &redeploy,
		); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		cred := getCred(thread, "")
		err := buddy.ProfileSetPayload(cred, scriptOpts, int(pid), string(domain), string(dictFile), bool(redeploy), logger)
		if err != nil {
			return resultErr(err.Error()), nil
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
			"profile_id", &profileID,
			"xml_file", &xmlFile,
			"redeploy?", &redeploy,
		); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		f, err := os.Open(string(xmlFile))
		if err != nil {
			return resultErr(fmt.Sprintf("open %s: %v", string(xmlFile), err)), nil
		}
		defer f.Close()
		cred := getCred(thread, "")
		err = buddy.ProfileSetXML(cred, scriptOpts, int(pid), f, bool(redeploy), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func profileClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var name, cred starlark.String
		if err := starlark.UnpackArgs("profile_clone", args, kwargs, "profile_id", &profileID, "name", &name, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		err := buddy.ProfileClone(getCred(thread, c), scriptOpts, int(pid), string(name), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// profile_fetch_xml(profile_id, cred?) → {ok, value: str}
func profileFetchXML(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var cred starlark.String
		if err := starlark.UnpackArgs("profile_fetch_xml", args, kwargs, "profile_id", &profileID, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		xmlBytes, err := buddy.ProfileFetchXML(getCred(thread, c), int(pid))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(string(xmlBytes))), nil
	}
}

// profile_put_xml(profile_id, xml_str, redeploy?, cred?)
func profilePutXML(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var xmlStr starlark.String
		redeploy := starlark.Bool(false)
		var cred starlark.String
		if err := starlark.UnpackArgs("profile_put_xml", args, kwargs,
			"profile_id", &profileID, "xml_str", &xmlStr, "redeploy?", &redeploy, "cred?", &cred); err != nil {
			return nil, err
		}
		pid, _ := profileID.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		credResolved := getCred(thread, c)
		err := buddy.ProfilePutXML(credResolved, scriptOpts, int(pid), []byte(string(xmlStr)), bool(redeploy), logger)
		if err != nil {
			return resultErr(err.Error()), nil
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
		return resultErr(err.Error()), nil
	}
	return resultOK(starlark.String(payloads)), nil
}

// profile_xml_replace_payloads(profile_xml_str, payloads_plist_str) → {ok, value: str}
func profileXMLReplacePayloadsBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var profileXMLStr, payloadsStr starlark.String
	if err := starlark.UnpackArgs("profile_xml_replace_payloads", args, kwargs,
		"profile_xml_str", &profileXMLStr, "payloads_plist_str", &payloadsStr); err != nil {
		return nil, err
	}
	out := buddy.ReplacePayloadsContent([]byte(string(profileXMLStr)), string(payloadsStr))
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

// payloads_list_identifiers(payloads_plist_str) → {ok, value: list of str}
func payloadsListIdentifiersBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr starlark.String
	if err := starlark.UnpackArgs("payloads_list_identifiers", args, kwargs, "payloads_plist_str", &plistStr); err != nil {
		return nil, err
	}
	ids, err := buddy.PlistPayloadIdentifiers(string(plistStr))
	if err != nil {
		return resultErr(err.Error()), nil
	}
	list := new(starlark.List)
	for _, id := range ids {
		_ = list.Append(starlark.String(id))
	}
	return resultOK(list), nil
}

// payloads_get_payload(payloads_plist_str, domain) → {ok, value: str}
func payloadsGetPayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr, domain starlark.String
	if err := starlark.UnpackArgs("payloads_get_payload", args, kwargs, "payloads_plist_str", &plistStr, "domain", &domain); err != nil {
		return nil, err
	}
	out, err := buddy.PlistGetPayloadByDomain(string(plistStr), string(domain))
	if err != nil {
		return resultErr(err.Error()), nil
	}
	return resultOK(starlark.String(string(out))), nil
}

// payloads_replace_payload(payloads_plist_str, domain, new_payload) → {ok, value: str}. new_payload is dict or plist XML string.
func payloadsReplacePayload(logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var plistStr, domain starlark.String
		var newPayload starlark.Value
		if err := starlark.UnpackArgs("payloads_replace_payload", args, kwargs,
			"payloads_plist_str", &plistStr, "domain", &domain, "new_payload", &newPayload); err != nil {
			return nil, err
		}
		var payloadMap interface{}
		switch v := newPayload.(type) {
		case *starlark.Dict:
			g, err := StarlarkValueToGo(v)
			if err != nil {
				return resultErr(err.Error()), nil
			}
			payloadMap = g
		case starlark.String:
			decoded, err := PlistDecodeToStarlark(string(v))
			if err != nil {
				return resultErr(err.Error()), nil
			}
			d, ok := decoded.(*starlark.Dict)
			if !ok {
				return resultErr("new_payload plist string must decode to a dict"), nil
			}
			g, err := StarlarkValueToGo(d)
			if err != nil {
				return resultErr(err.Error()), nil
			}
			payloadMap = g
		default:
			return resultErr("new_payload must be dict or plist XML string"), nil
		}
		updated, err := buddy.PlistReplacePayloadByDomain(string(plistStr), string(domain), payloadMap)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(updated)), nil
	}
}

// payloads_replace_mcx_dict(payloads_plist_str, domain, dict_xml_str) → {ok, value: str}
func payloadsReplaceMCXDictBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plistStr, domain, dictXML starlark.String
	if err := starlark.UnpackArgs("payloads_replace_mcx_dict", args, kwargs,
		"payloads_plist_str", &plistStr, "domain", &domain, "dict_xml_str", &dictXML); err != nil {
		return nil, err
	}
	updated, err := buddy.ReplaceMCXDict(string(plistStr), string(domain), string(dictXML))
	if err != nil {
		return resultErr(err.Error()), nil
	}
	return resultOK(starlark.String(updated)), nil
}

func stepSetProfilePayloadBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var profileID starlark.Int
	var domain, dictFile starlark.String
	redeploy := starlark.Bool(false)
	if err := starlark.UnpackArgs("step_set_profile_payload", args, kwargs,
		"profile_id", &profileID,
		"domain", &domain,
		"dict_file", &dictFile,
		"redeploy?", &redeploy,
	); err != nil {
		return nil, err
	}
	pid, _ := profileID.Int64()
	d := stepDict(stepTypeSetProfilePayload,
		"profile_id", int(pid),
		"domain", string(domain),
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
		"profile_id", &profileID,
		"xml_file", &xmlFile,
		"redeploy?", &redeploy,
	); err != nil {
		return nil, err
	}
	pid, _ := profileID.Int64()
	d := stepDict(stepTypeSetProfileXML,
		"profile_id", int(pid),
		"xml_file", string(xmlFile),
		"redeploy", bool(redeploy),
	)
	return d, nil
}

// script_fetch(script_id, cred?) → {ok, value: dict}
func scriptFetch(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID, cred starlark.String
		if err := starlark.UnpackArgs("script_fetch", args, kwargs, "script_id", &scriptID, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		s, err := buddy.ScriptFetchFull(getCred(thread, c), string(scriptID))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, mErr := json.Marshal(s)
		if mErr != nil {
			return resultErr(mErr.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// script_put(script_id, dict, cred?) → {ok, value: dict}
func scriptPutBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID starlark.String
		var bodyVal starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("script_put", args, kwargs, "script_id", &scriptID, "dict", &bodyVal, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict"), nil
		}
		updated, err := buddy.ScriptPutFields(getCred(thread, c), scriptOpts, string(scriptID), m, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, mErr := json.Marshal(updated)
		if mErr != nil {
			return resultErr(mErr.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(b))
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(val), nil
	}
}

// ---------------------------------------------------------------------------
// Computer builtins
// ---------------------------------------------------------------------------

// computer_find_ids(filter, cred?) → {ok, value: [id1, id2, ...]} or {ok: False, error: "..."}
func computerFindIDs(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter, cred starlark.String
		if err := starlark.UnpackArgs("computer_find_ids", args, kwargs, "filter?", &filter, "cred?", &cred); err != nil {
			return nil, err
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		credResolved := getCred(thread, c)
		records, err := buddy.ComputerSearch(credResolved, string(filter), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := starlark.NewList(nil)
		for _, r := range records {
			_ = list.Append(starlark.MakeInt(r.ID))
		}
		return resultOK(list), nil
	}
}

// computer_get(computer_id, cred?) → {ok, value: dict} – id, name, serial, managementId, etc.
func computerGet(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDVal starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_get", args, kwargs, "computer_id", &computerIDVal, "cred?", &cred); err != nil {
			return nil, err
		}
		var computerIDStr string
		switch v := computerIDVal.(type) {
		case starlark.Int:
			n, _ := v.Int64()
			computerIDStr = strconv.FormatInt(n, 10)
		case starlark.String:
			computerIDStr = string(v)
		default:
			return nil, fmt.Errorf("computer_get: computer_id must be int or string")
		}
		c := credName
		if cred != "" {
			c = string(cred)
		}
		credResolved := getCred(thread, c)
		rec, err := buddy.ComputerGetValue(credResolved, computerIDStr, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		d := new(starlark.Dict)
		for k, v := range rec {
			var sv starlark.Value
			switch tv := v.(type) {
			case int:
				sv = starlark.MakeInt(tv)
			case string:
				sv = starlark.String(tv)
			default:
				sv = starlark.String(fmt.Sprint(v))
			}
			_ = d.SetKey(starlark.String(k), sv)
		}
		return resultOK(d), nil
	}
}

// computer_reboot(computer_ids, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerReboot(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_reboot", args, kwargs, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_reboot")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerReboot(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_redeploy(credName, computer_ids) → {ok, value: None} or {ok: False, error: "..."}
func computerRedeploy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_redeploy", args, kwargs, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_redeploy")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerRedeploy(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func parseComputerIDsAndCred(computerIDs *starlark.List, cred starlark.String, credName string, thread *starlark.Thread, fn string) ([]int, string, error) {
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
	c := credName
	if cred != "" {
		c = string(cred)
	}
	return ids, getCred(thread, c), nil
}

// computer_send_mdm_command(computer_ids, command_type, command_data?, cred?) → {ok, value: None}
func computerSendMDMCommand(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var commandType starlark.String
		var commandDataVal starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_send_mdm_command", args, kwargs,
			"computer_ids", &computerIDs,
			"command_type", &commandType,
			"command_data?", &commandDataVal,
			"cred?", &cred,
		); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_send_mdm_command")
		if err != nil {
			return nil, err
		}
		var cmdData map[string]interface{}
		if commandDataVal != nil && commandDataVal != starlark.None {
			g, err := StarlarkValueToGo(commandDataVal)
			if err != nil {
				return resultErr(err.Error()), nil
			}
			m, ok := g.(map[string]interface{})
			if !ok {
				return resultErr("command_data must be a dict"), nil
			}
			cmdData = m
		}
		if err := buddy.ComputerSendMDMCommand(c, ids, string(commandType), cmdData, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_shutdown(computer_ids, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerShutdown(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_shutdown", args, kwargs, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_shutdown")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerShutdown(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_lock(computer_ids, message?, phone?, pin?, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerLock(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var message, phone, pin, cred starlark.String
		if err := starlark.UnpackArgs("computer_lock", args, kwargs,
			"computer_ids", &computerIDs,
			"message?", &message, "phone?", &phone, "pin?", &pin, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_lock")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerLock(c, scriptOpts, ids, string(message), string(phone), string(pin), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_logout(computer_ids, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerLogout(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_logout", args, kwargs, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_logout")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerLogOut(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_erase(computer_ids, pin?, obliteration?, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerErase(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var pin, obliteration, cred starlark.String
		if err := starlark.UnpackArgs("computer_erase", args, kwargs,
			"computer_ids", &computerIDs, "pin?", &pin, "obliteration?", &obliteration, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_erase")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerErase(c, scriptOpts, ids, string(pin), string(obliteration), false, false, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_device_info(computer_ids, queries?, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerDeviceInfo(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var queries *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_device_info", args, kwargs,
			"computer_ids", &computerIDs, "queries?", &queries, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_device_info")
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
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_device_location(computer_ids, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerDeviceLocation(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_device_location", args, kwargs, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_device_location")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerDeviceLocation(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_security_info(computer_ids, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerSecurityInfo(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_security_info", args, kwargs, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_security_info")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerSecurityInfo(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_installed_apps(computer_ids, identifiers?, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerInstalledApps(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var identifiers *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_installed_apps", args, kwargs,
			"computer_ids", &computerIDs, "identifiers?", &identifiers, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_installed_apps")
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
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_managed_apps(computer_ids, identifiers?, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerManagedApps(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var identifiers *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_managed_apps", args, kwargs,
			"computer_ids", &computerIDs, "identifiers?", &identifiers, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_managed_apps")
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
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// computer_certificates(computer_ids, cred?) → {ok, value: None} or {ok: False, error: "..."}
func computerCertificates(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var computerIDs *starlark.List
		var cred starlark.String
		if err := starlark.UnpackArgs("computer_certificates", args, kwargs, "computer_ids", &computerIDs, "cred?", &cred); err != nil {
			return nil, err
		}
		ids, c, err := parseComputerIDsAndCred(computerIDs, cred, credName, thread, "computer_certificates")
		if err != nil {
			return nil, err
		}
		if err := buddy.ComputerCertificates(c, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// ---------------------------------------------------------------------------
// MDM builtins
// ---------------------------------------------------------------------------

// mdm_command_find(filter, sort="dateSent:asc", page_size=200, limit=0, cred=None) → {ok, value: list of dicts}
// Each dict has keys: uuid, command_type, command_state, client_management_id,
// client_type, date_sent, date_completed.
// page_size controls records per API request; limit caps the total returned (0 = no cap).
func mdmCommandFind(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sort starlark.String
		var pageSize starlark.Int
		var limit starlark.Int
		var cred starlark.String
		if err := starlark.UnpackArgs("mdm_command_find", args, kwargs,
			"filter", &filter, "sort?", &sort, "page_size?", &pageSize, "limit?", &limit, "cred?", &cred,
		); err != nil {
			return nil, err
		}
		sortStr := string(sort)
		if sortStr == "" {
			sortStr = "dateSent:asc"
		}
		ps, _ := pageSize.Int64()
		lim, _ := limit.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		records, err := buddy.MdmCommandFind(getCred(thread, c), string(filter), sortStr, int(ps), int(lim), logger)
		if err != nil {
			return resultErr(err.Error()), nil
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

// mdm_fetch_commands(filter, sort="dateSent:asc", page_size=200, limit=0, cred=None) → {ok, value: list of dicts}
// Alias of mdm_command_find with a more \"component\"-style name.
func mdmFetchCommands(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sort starlark.String
		var pageSize starlark.Int
		var limit starlark.Int
		var cred starlark.String
		if err := starlark.UnpackArgs("mdm_fetch_commands", args, kwargs,
			"filter", &filter, "sort?", &sort, "page_size?", &pageSize, "limit?", &limit, "cred?", &cred,
		); err != nil {
			return nil, err
		}
		// Delegate to the same implementation used by mdm_command_find.
		sortStr := string(sort)
		if sortStr == "" {
			sortStr = "dateSent:asc"
		}
		ps, _ := pageSize.Int64()
		lim, _ := limit.Int64()
		c := credName
		if cred != "" {
			c = string(cred)
		}
		records, err := buddy.MdmCommandFind(getCred(thread, c), string(filter), sortStr, int(ps), int(lim), logger)
		if err != nil {
			return resultErr(err.Error()), nil
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
// run_workflow builtin
// ---------------------------------------------------------------------------

func runWorkflowBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var steps *starlark.List
		if err := starlark.UnpackArgs("run_workflow", args, kwargs, "steps", &steps); err != nil {
			return nil, err
		}
		cred := getCred(thread, credName)
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
