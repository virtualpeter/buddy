package script

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

// ---------------------------------------------------------------------------
// v3 plural / singular builtins
// ---------------------------------------------------------------------------

// policies(keywords?, exclude_ids?, force_refresh?, ids?=False, tenant?, surface?, auth?) → list of {id,name} or list of int
func policiesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var keywordsVal starlark.Value = starlark.None
		var excludeIDs *starlark.List
		var forceRefresh, idsOnly starlark.Bool
		var sess sessionArgs
		if err := starlark.UnpackArgs("policies", args, kwargs,
			"keywords?", &keywordsVal, "exclude_ids?", &excludeIDs, "force_refresh?", &forceRefresh, "ids?", &idsOnly,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		var kw []string
		if keywordsVal != nil && keywordsVal != starlark.None {
			var err error
			kw, err = stringListFromStarlark(keywordsVal, "keywords")
			if err != nil {
				return nil, err
			}
		}
		exclude, err := intListFromStarlark(excludeIDs, "exclude_ids")
		if err != nil {
			return nil, err
		}
		summaries, err := buddy.PolicyFindSummariesByKeywords(c, scriptOpts, kw, exclude, bool(forceRefresh), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := starlark.NewList(nil)
		if bool(idsOnly) {
			for _, s := range summaries {
				_ = list.Append(starlark.MakeInt(s.ID))
			}
		} else {
			for _, s := range summaries {
				d := new(starlark.Dict)
				_ = d.SetKey(starlark.String("id"), starlark.MakeInt(s.ID))
				_ = d.SetKey(starlark.String("name"), starlark.String(s.Name))
				_ = list.Append(d)
			}
		}
		return resultOK(list), nil
	}
}

// policy(id=?) | policy(trigger=?, force_refresh?=) → details dict
func policyBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var trigger starlark.String
		var forceRefresh starlark.Bool
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy", args, kwargs,
			"id?", &idVal, "trigger?", &trigger, "force_refresh?", &forceRefresh,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		idSet := idVal != nil && idVal != starlark.None
		keySet := trigger != ""
		if err := xorIDOrKey("policy", idSet, keySet, "trigger"); err != nil {
			return nil, err
		}
		var pid int
		if idSet {
			var err error
			pid, err = starlarkIDInt(idVal, "id")
			if err != nil {
				return nil, err
			}
		} else {
			id, err := buddy.PolicyFind(c, scriptOpts, string(trigger), bool(forceRefresh), logger)
			if err != nil {
				return resultErr(err.Error()), nil
			}
			pid = id
		}
		d, err := buddy.PolicyGetDetails(c, pid, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(policyDetailsToStarlark(d)), nil
	}
}

// policy_xml(id, tenant?, surface?, auth?) → XML string
func policyXMLBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_xml", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		pid, err := starlarkIDInt(idVal, "id")
		if err != nil {
			return nil, err
		}
		xmlBytes, err := buddy.PolicyFetchXML(c, pid)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(string(xmlBytes))), nil
	}
}

// policy_set(policy_id, enable?=, name?=, trigger?=, frequency?=, tenant?, surface?, auth?)
func policySetBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var policyID starlark.Int
		var enable *starlark.Bool
		var name, trigger, frequency starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_set", args, kwargs,
			"policy_id", &policyID,
			"enable?", &enable, "name?", &name, "trigger?", &trigger, "frequency?", &frequency,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		if enable == nil && name == "" && trigger == "" && frequency == "" {
			return nil, fmt.Errorf("policy_set: pass at least one of enable=, name=, trigger=, frequency=")
		}
		pid, _ := policyID.Int64()
		if enable != nil {
			if err := buddy.PolicyEnable(c, scriptOpts, int(pid), bool(*enable), logger); err != nil {
				return resultErr(err.Error()), nil
			}
		}
		if name != "" {
			if err := buddy.PolicySetName(c, scriptOpts, int(pid), string(name), logger); err != nil {
				return resultErr(err.Error()), nil
			}
		}
		if trigger != "" {
			if err := buddy.PolicyTrigger(c, scriptOpts, int(pid), string(trigger), logger); err != nil {
				return resultErr(err.Error()), nil
			}
		}
		if frequency != "" {
			if err := buddy.PolicySetFrequency(c, scriptOpts, int(pid), string(frequency), logger); err != nil {
				return resultErr(err.Error()), nil
			}
		}
		return resultOK(starlark.None), nil
	}
}

// packages(...) — enumerate package summaries
func packagesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("packages", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		results, err := buddy.PackageSearch(c, string(filter), opts, logger)
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

// package(id=?) | package(app=, version=, pattern?=) → full dict or {id,name}
func packageBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var app, version, pattern starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("package", args, kwargs,
			"id?", &idVal, "app?", &app, "version?", &version, "pattern?", &pattern,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		idSet := idVal != nil && idVal != starlark.None
		keySet := app != "" || version != ""
		if idSet && keySet {
			return nil, fmt.Errorf("package: pass either id= or app=/version=, not both")
		}
		if !idSet && !keySet {
			return nil, fmt.Errorf("package: require id= or app= and version=")
		}
		if idSet {
			id, err := starlarkIDString(idVal, "id")
			if err != nil {
				return nil, err
			}
			pkg, err := buddy.PackageFetchFull(c, id)
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
		if app == "" || version == "" {
			return nil, fmt.Errorf("package: app= and version= are both required when not using id=")
		}
		patternOverride := ""
		if pattern != "" {
			patternOverride = string(pattern)
		}
		id, name, err := buddy.PackageFindID(c, string(app), string(version), patternOverride, logger)
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

// scripts(...)
func scriptsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("scripts", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		results, err := buddy.ScriptSearch(c, string(filter), opts, logger)
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

// script(id=?) | script(name=?) → full dict or id string
func scriptBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script", args, kwargs, "id?", &idVal, "name?", &name,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		idSet := idVal != nil && idVal != starlark.None
		keySet := name != ""
		if err := xorIDOrKey("script", idSet, keySet, "name"); err != nil {
			return nil, err
		}
		if keySet {
			id, err := buddy.ScriptGetID(c, string(name), logger)
			if err != nil {
				return resultErr(err.Error()), nil
			}
			return resultOK(starlark.String(id)), nil
		}
		id, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		s, err := buddy.ScriptFetchFull(c, id)
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

// computers(filter?, ids?=False, page?, page_size?, tenant?, surface?, auth?)
func computersBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var idsOnly starlark.Bool
		var sess sessionArgs
		if err := starlark.UnpackArgs("computers", args, kwargs,
			"filter?", &filter, "ids?", &idsOnly, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		records, err := buddy.ComputerSearch(c, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		list := starlark.NewList(nil)
		if bool(idsOnly) {
			for _, r := range records {
				_ = list.Append(starlark.MakeInt(r.ID))
			}
		} else {
			for _, r := range records {
				d := new(starlark.Dict)
				_ = d.SetKey(starlark.String("id"), starlark.MakeInt(r.ID))
				_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
				_ = d.SetKey(starlark.String("serial"), starlark.String(r.SerialNumber))
				_ = d.SetKey(starlark.String("serialNumber"), starlark.String(r.SerialNumber))
				_ = d.SetKey(starlark.String("udid"), starlark.String(r.UDID))
				_ = d.SetKey(starlark.String("managementId"), starlark.String(r.ManagementID))
				_ = d.SetKey(starlark.String("macAddress"), starlark.String(r.MacAddress))
				_ = d.SetKey(starlark.String("username"), starlark.String(r.Username))
				_ = list.Append(d)
			}
		}
		return resultOK(list), nil
	}
}

// computer(id, tenant?, surface?, auth?)
func computerBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		idStr, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		rec, err := buddy.ComputerGetValue(c, idStr, logger)
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

// mdm_commands(...)
func mdmCommandsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter, sort starlark.String
		page := starlark.Value(starlark.None)
		var pageSize, limit starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("mdm_commands", args, kwargs,
			"filter", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize, "limit?", &limit,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
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

// profiles(filter?)
func profilesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("profiles", args, kwargs, "filter?", &filter); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		records, err := buddy.ProfileSearch(c, string(filter), logger)
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

// profile(id=?) → XML | profile(name=?) → {id,name}
func profileBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile", args, kwargs, "id?", &idVal, "name?", &name,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		idSet := idVal != nil && idVal != starlark.None
		keySet := name != ""
		if err := xorIDOrKey("profile", idSet, keySet, "name"); err != nil {
			return nil, err
		}
		if keySet {
			id, profileName, err := buddy.ProfileFindID(c, string(name), logger)
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
		pid, err := starlarkIDInt(idVal, "id")
		if err != nil {
			return nil, err
		}
		xmlBytes, err := buddy.ProfileFetchXML(c, pid)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(string(xmlBytes))), nil
	}
}

// profile_xml(id, tenant?, surface?, auth?)
func profileXMLBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("profile_xml", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		pid, err := starlarkIDInt(idVal, "id")
		if err != nil {
			return nil, err
		}
		xmlBytes, err := buddy.ProfileFetchXML(c, pid)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(string(xmlBytes))), nil
	}
}

// smart_groups / static_groups
func smartGroupsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return groupListBuiltin("smart_groups", buddy.GroupSearchSmart, credName, logger)
}

func staticGroupsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return groupListBuiltin("static_groups", buddy.GroupSearchStatic, credName, logger)
}

// smart_group(id=?) | smart_group(name=?) → full JSON dict
func smartGroupBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group", args, kwargs, "id?", &idVal, "name?", &name,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		idSet := idVal != nil && idVal != starlark.None
		keySet := name != ""
		if err := xorIDOrKey("smart_group", idSet, keySet, "name"); err != nil {
			return nil, err
		}
		var groupID string
		if keySet {
			id, _, err := buddy.GroupFindID(c, string(name), logger)
			if err != nil {
				return resultErr(err.Error()), nil
			}
			if id == "" {
				return resultErr(fmt.Sprintf("no smart group found for name %q", string(name))), nil
			}
			groupID = id
		} else {
			var err error
			groupID, err = starlarkIDString(idVal, "id")
			if err != nil {
				return nil, err
			}
		}
		body, err := buddy.GroupFetchSmartRaw(c, groupID)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		val, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(fmt.Sprintf("decode smart group JSON: %v", err)), nil
		}
		return resultOK(val), nil
	}
}

// static_group(id) → {id, name, description}
func staticGroupBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("static_group", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		wantID, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		groups, err := buddy.GroupSearchStatic(c, "", buddy.PageOpts{Page: buddy.PageAll}, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		for _, g := range groups {
			if g.ID == wantID {
				d := new(starlark.Dict)
				_ = d.SetKey(starlark.String("id"), starlark.String(g.ID))
				_ = d.SetKey(starlark.String("name"), starlark.String(g.Name))
				_ = d.SetKey(starlark.String("description"), starlark.String(g.Description))
				return resultOK(d), nil
			}
		}
		return resultErr(fmt.Sprintf("no static group found for id %q", wantID)), nil
	}
}

// roles(full?=False, page?, page_size?, tenant?, surface?, auth?)
func rolesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var full starlark.Bool
		var sess sessionArgs
		if err := starlark.UnpackArgs("roles", args, kwargs, "full?", &full, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		roles, err := buddy.RoleListValues(c, opts, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if bool(full) {
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

// role(id, tenant?, surface?, auth?)
func roleBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("role", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		roleID, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		role, err := buddy.RoleFetch(c, roleID, logger)
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

// role_privileges — alias of role_privileges_list under shorter name
func rolePrivilegesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	inner := rolePrivilegesList(credName, logger)
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return inner(thread, b, args, kwargs)
	}
}

// clients(page?, page_size?, tenant?, surface?, auth?)
func clientsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("clients", args, kwargs, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		clients, err := buddy.ClientListValues(c, opts, logger)
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

// client(id, tenant?, surface?, auth?)
func clientBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("client", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		clientID, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		client, err := buddy.ClientFetch(c, clientID, logger)
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

func smartGroupGetVersionNamed(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_get_version", args, kwargs, "group_id", &groupID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		version, err := buddy.GroupGetVersionValue(c, string(groupID), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.String(version)), nil
	}
}

func smartGroupSetVersionNamed(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, version starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("smart_group_set_version", args, kwargs, "group_id", &groupID, "version", &version,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		err := buddy.GroupVersion(c, scriptOpts, string(groupID), string(version), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

// blueprints(search?, page?, page_size?, tenant?, surface?, auth?) → list of summary dicts
func blueprintsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var search starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprints", args, kwargs, "search?", &search, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		items, err := buddy.BlueprintListValues(c, buddy.BlueprintListOpts{
			Search: string(search),
			Page:   opts,
		}, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(items)
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

// blueprint(id=, tenant?, surface?, auth?) → full dict
func blueprintBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint", args, kwargs, "id", &id,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		out, err := buddy.BlueprintGetValue(c, string(id), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(out)
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

func blueprintCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_create", args, kwargs, "body", &body,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		out, err := buddy.BlueprintCreateValue(c, scriptOpts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if out == nil {
			return resultErr("blueprint_create: cancelled"), nil
		}
		b, err := json.Marshal(out)
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

func blueprintSetBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var blueprintID starlark.String
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_set", args, kwargs, "blueprint_id", &blueprintID, "body", &body,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		if err := buddy.BlueprintSetValue(c, string(blueprintID), scriptOpts, []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func blueprintDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var blueprintID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_delete", args, kwargs, "blueprint_id", &blueprintID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		if err := buddy.BlueprintDeleteValue(c, string(blueprintID), scriptOpts, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func blueprintDeployBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var blueprintID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_deploy", args, kwargs, "blueprint_id", &blueprintID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		if err := buddy.BlueprintDeploy(c, string(blueprintID), scriptOpts, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func blueprintUndeployBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var blueprintID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_undeploy", args, kwargs, "blueprint_id", &blueprintID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		if err := buddy.BlueprintUndeploy(c, string(blueprintID), scriptOpts, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func blueprintStatusBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var blueprintID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_status", args, kwargs, "blueprint_id", &blueprintID,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		out, err := buddy.BlueprintStatusValue(c, string(blueprintID), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(out)
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

func blueprintComponentsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_components", args, kwargs, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		items, err := buddy.BlueprintComponentsValues(c, opts, logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(items)
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

func blueprintComponentBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("blueprint_component", args, kwargs, "id", &id,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		out, err := buddy.BlueprintComponentValue(c, string(id), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		b, err := json.Marshal(out)
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
