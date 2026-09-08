package script

import (
	"log/slog"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerMobileSearchBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "mobile_searches", mobileSearchesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_search", mobileSearchBuiltin(credName, logger))
	putBuiltin(dict, "mobile_search_devices", mobileSearchDevicesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_search_create", mobileSearchCreateBuiltin(credName, logger))
	putBuiltin(dict, "mobile_search_put", mobileSearchPutBuiltin(credName, logger))
	putBuiltin(dict, "mobile_search_delete", mobileSearchDeleteBuiltin(credName, logger))
}

func mobileSearchSummaryList(rows []buddy.MobileSearchSummary) starlark.Value {
	list := starlark.NewList(nil)
	for _, r := range rows {
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(r.ID))
		_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
		_ = list.Append(d)
	}
	return list
}

func mobileSearchesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_searches", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		rows, err := buddy.MobileSearchSearch(pin, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(mobileSearchSummaryList(rows)), nil
	}
}

func mobileSearchBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_search", args, kwargs, "id?", &idVal, "name?", &name,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		nameStr := string(name)
		if err := xorIDOrKey("mobile_search", idSet, nameStr != "", "name"); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		var d map[string]interface{}
		var err error
		if idSet {
			idStr, idErr := starlarkIDString(idVal, "id")
			if idErr != nil {
				return nil, idErr
			}
			d, err = buddy.MobileSearchFetch(pin, idStr)
		} else {
			d, err = buddy.MobileSearchFetchByName(pin, nameStr)
		}
		if err != nil {
			return resultErr(err.Error())
		}
		return jsonResult(d, nil)
	}
}

func mobileSearchDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_search_devices", args, kwargs, "id", &idVal, "filter?", &filter,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idStr, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		rows, err := buddy.MobileSearchDevices(pin, idStr, string(filter))
		if err != nil {
			return resultErr(err.Error())
		}
		return jsonResult(rows, nil)
	}
}

func mobileSearchCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_search_create", args, kwargs, "body", &body,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		m, err := bodyMapFromStarlark(body, "body")
		if err != nil {
			return resultErr(err.Error())
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		sum, err := buddy.MobileSearchCreate(pin, scriptOpts, m, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if sum == nil {
			return resultErr("mobile_search_create: cancelled")
		}
		return jsonResult(sum, nil)
	}
}

func mobileSearchPutBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal, body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_search_put", args, kwargs, "id", &idVal, "body", &body,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idStr, err := starlarkIDString(idVal, "id")
		if err != nil {
			return nil, err
		}
		m, err := bodyMapFromStarlark(body, "body")
		if err != nil {
			return resultErr(err.Error())
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		if err := buddy.MobileSearchPut(pin, scriptOpts, idStr, m, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func mobileSearchDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal, idsVal starlark.Value = starlark.None, starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_search_delete", args, kwargs, "id?", &idVal, "ids?", &idsVal,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		idsSet := idsVal != nil && idsVal != starlark.None
		if err := xorIDOrKey("mobile_search_delete", idSet, idsSet, "ids"); err != nil {
			return nil, err
		}
		var ids []string
		var err error
		if idSet {
			ids, err = idListFromStarlark(idVal, "id")
		} else {
			ids, err = idListFromStarlark(idsVal, "ids")
		}
		if err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		if err := buddy.MobileSearchDelete(pin, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}
