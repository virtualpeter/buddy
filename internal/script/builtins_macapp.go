package script

import (
	"encoding/json"
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerMacAppBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "macapps", macappsBuiltin(credName, logger))
	putBuiltin(dict, "macapp", macappBuiltin(credName, logger))
	putBuiltin(dict, "macapp_create", macappCreateBuiltin(credName, logger))
	putBuiltin(dict, "macapp_delete", macappDeleteBuiltin(credName, logger))
}

func macappsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("macapps", args, kwargs,
			"filter?", &filter,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		apps, err := buddy.MacAppSearch(pin, string(filter), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := make([]starlark.Value, 0, len(apps))
		for _, a := range apps {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.MakeInt(a.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(a.Name))
			_ = d.SetKey(starlark.String("version"), starlark.String(a.Version))
			_ = d.SetKey(starlark.String("bundle_id"), starlark.String(a.BundleID))
			list = append(list, d)
		}
		return resultOK(starlark.NewList(list)), nil
	}
}

func macappBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("macapp", args, kwargs,
			"id?", &idVal, "name?", &name,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		nameStr := strings.TrimSpace(string(name))
		if err := xorIDOrKey("macapp", idSet, nameStr != "", "name"); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		var d *buddy.MacAppDetail
		var err error
		if idSet {
			idStr, idErr := starlarkIDString(idVal, "id")
			if idErr != nil {
				return nil, idErr
			}
			d, err = buddy.MacAppFetch(pin, idStr)
		} else {
			d, err = buddy.MacAppFetchByName(pin, nameStr)
		}
		if err != nil {
			return resultErr(err.Error())
		}
		return macAppDetailResult(d)
	}
}

func macappCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, version, bundleID, url, category starlark.String
		free := starlark.Bool(false)
		var sess sessionArgs
		if err := starlark.UnpackArgs("macapp_create", args, kwargs,
			"name", &name, "version", &version, "bundle_id", &bundleID, "url", &url,
			"free?", &free, "category?", &category,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		sum, err := buddy.MacAppCreate(pin, scriptOpts, buddy.MacAppCreateSpec{
			Name:     string(name),
			Version:  string(version),
			BundleID: string(bundleID),
			URL:      string(url),
			IsFree:   bool(free),
			Category: string(category),
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if sum == nil {
			return resultErr("macapp_create: cancelled")
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.MakeInt(sum.ID))
		_ = d.SetKey(starlark.String("name"), starlark.String(sum.Name))
		return resultOK(d), nil
	}
}

func macappDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var macappID starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("macapp_delete", args, kwargs,
			"macapp_id", &macappID,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		id, err := starlarkIDInt(macappID, "macapp_id")
		if err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		if err := buddy.MacAppDelete(pin, scriptOpts, []int{id}, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func macAppDetailResult(d *buddy.MacAppDetail) (starlark.Value, error) {
	out := map[string]interface{}{
		"id":        d.ID,
		"name":      d.Name,
		"version":   d.Version,
		"bundle_id": d.BundleID,
		"url":       d.URL,
		"is_free":   d.IsFree,
	}
	if len(d.General) > 0 {
		var g interface{}
		if err := json.Unmarshal(d.General, &g); err != nil {
			return resultErr(err.Error())
		}
		out["general"] = g
	}
	if len(d.Scope) > 0 {
		var s interface{}
		if err := json.Unmarshal(d.Scope, &s); err != nil {
			return resultErr(err.Error())
		}
		out["scope"] = s
	}
	if len(d.SelfService) > 0 {
		var ss interface{}
		if err := json.Unmarshal(d.SelfService, &ss); err != nil {
			return resultErr(err.Error())
		}
		out["self_service"] = ss
	}
	return jsonResult(out, nil)
}
