package script

import (
	"fmt"
	"log/slog"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerComputerSearchBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "computer_searches", computerSearchesBuiltin(credName, logger))
	putBuiltin(dict, "computer_search", computerSearchBuiltin(credName, logger))
	putBuiltin(dict, "computer_search_computers", computerSearchComputersBuiltin(credName, logger))
	putBuiltin(dict, "computer_search_create", computerSearchCreateBuiltin(credName, logger))
	putBuiltin(dict, "computer_search_put", computerSearchPutBuiltin(credName, logger))
	putBuiltin(dict, "computer_search_delete", computerSearchDeleteBuiltin(credName, logger))
}

func bodyMapFromStarlark(v starlark.Value, label string) (map[string]interface{}, error) {
	g, err := StarlarkValueToGo(v)
	if err != nil {
		return nil, err
	}
	m, ok := g.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a dict", label)
	}
	return m, nil
}

func searchSummaryList(rows []buddy.ComputerSearchSummary) starlark.Value {
	list := starlark.NewList(nil)
	for _, r := range rows {
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(r.ID))
		_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
		_ = list.Append(d)
	}
	return list
}

func computerSearchesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_searches", args, kwargs, "filter?", &filter,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		rows, err := buddy.ComputerSearchSearch(pin, string(filter), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(searchSummaryList(rows)), nil
	}
}

func computerSearchBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_search", args, kwargs, "id?", &idVal, "name?", &name,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		nameStr := string(name)
		if err := xorIDOrKey("computer_search", idSet, nameStr != "", "name"); err != nil {
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
			d, err = buddy.ComputerSearchFetch(pin, idStr)
		} else {
			d, err = buddy.ComputerSearchFetchByName(pin, nameStr)
		}
		if err != nil {
			return resultErr(err.Error())
		}
		return jsonResult(d, nil)
	}
}

func computerSearchComputersBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value
		var filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_search_computers", args, kwargs, "id", &idVal, "filter?", &filter,
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
		rows, err := buddy.ComputerSearchComputers(pin, idStr, string(filter))
		if err != nil {
			return resultErr(err.Error())
		}
		return jsonResult(rows, nil)
	}
}

func computerSearchCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_search_create", args, kwargs, "body", &body,
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
		sum, err := buddy.ComputerSearchCreate(pin, scriptOpts, m, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if sum == nil {
			return resultErr("computer_search_create: cancelled")
		}
		return jsonResult(sum, nil)
	}
}

func computerSearchPutBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal, body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_search_put", args, kwargs, "id", &idVal, "body", &body,
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
		if err := buddy.ComputerSearchPut(pin, scriptOpts, idStr, m, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func computerSearchDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal, idsVal starlark.Value = starlark.None, starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("computer_search_delete", args, kwargs, "id?", &idVal, "ids?", &idsVal,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		idsSet := idsVal != nil && idsVal != starlark.None
		if err := xorIDOrKey("computer_search_delete", idSet, idsSet, "ids"); err != nil {
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
		if err := buddy.ComputerSearchDelete(pin, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}
