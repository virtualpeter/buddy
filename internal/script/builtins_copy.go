package script

import (
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerCopyBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "categories", categoriesBuiltin(credName, logger))
	putBuiltin(dict, "category", categoryBuiltin(credName, logger))
	putBuiltin(dict, "category_copy", categoryCopy(credName, logger))
	putBuiltin(dict, "category_clone", categoryClone(credName, logger))
	putBuiltin(dict, "category_migrate", categoryMigrate(credName, logger))
	putBuiltin(dict, "script_copy", scriptCopy(credName, logger))
	putBuiltin(dict, "script_clone", scriptClone(credName, logger))
	putBuiltin(dict, "script_migrate", scriptMigrate(credName, logger))
	putBuiltin(dict, "smart_group_copy", smartGroupCopy(credName, logger))
	putBuiltin(dict, "smart_group_clone", smartGroupClone(credName, logger))
	putBuiltin(dict, "smart_group_migrate", smartGroupMigrate(credName, logger))
}

func categoriesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var page starlark.Value = starlark.None
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("categories", args, kwargs,
			"filter?", &filter, "page?", &page, "page_size?", &pageSize,
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
			return resultErr(err.Error())
		}
		cats, err := buddy.CategorySearch(pin, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := make([]starlark.Value, 0, len(cats))
		for _, c := range cats {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(c.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(c.Name))
			_ = d.SetKey(starlark.String("priority"), starlark.MakeInt(c.Priority))
			list = append(list, d)
		}
		return resultOK(starlark.NewList(list)), nil
	}
}

func categoryBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id, name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("category", args, kwargs,
			"id?", &id, "name?", &name,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idStr := strings.TrimSpace(string(id))
		nameStr := strings.TrimSpace(string(name))
		if (idStr == "") == (nameStr == "") {
			return resultErr("pass either id= or name=, not both")
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		var c *buddy.CategoryDetail
		var err error
		if idStr != "" {
			c, err = buddy.CategoryFetch(pin, idStr)
		} else {
			var found bool
			c, found, err = buddy.CategoryFindByExactName(pin, nameStr)
			if err == nil && !found {
				err = fmt.Errorf("category %q not found", nameStr)
			}
		}
		if err != nil {
			return resultErr(err.Error())
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(c.ID))
		_ = d.SetKey(starlark.String("name"), starlark.String(c.Name))
		_ = d.SetKey(starlark.String("priority"), starlark.MakeInt(c.Priority))
		return resultOK(d), nil
	}
}

func categoryCopy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var categoryID, name starlark.String
		var priority starlark.Value = starlark.None
		upsert := starlark.Bool(false)
		var dual dualSessionKwargs
		tail := append([]any{
			"category_id", &categoryID, "name?", &name, "priority?", &priority, "upsert?", &upsert,
		}, dual.unpackTail()...)
		if err := starlark.UnpackArgs("category_copy", args, kwargs, tail...); err != nil {
			return nil, err
		}
		src, dst, err := dual.refs(thread, credName)
		if err != nil {
			return resultErr(err.Error())
		}
		pri, err := optionalIntPtr(priority)
		if err != nil {
			return resultErr(err.Error())
		}
		if bool(upsert) {
			out, err := buddy.CategoryMigrate(src, dst, scriptOpts, buddy.CategoryMigrateSpec{
				SourceCategoryID: string(categoryID),
				Name:             string(name),
				Priority:         pri,
			}, logger)
			if err != nil {
				return resultErr(err.Error())
			}
			if out == nil {
				return resultErr("category_copy: cancelled")
			}
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(out.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(out.Name))
			_ = d.SetKey(starlark.String("priority"), starlark.MakeInt(out.Priority))
			_ = d.SetKey(starlark.String("created"), starlark.Bool(out.Created))
			return resultOK(d), nil
		}
		if strings.TrimSpace(string(name)) == "" {
			return resultErr("category_copy: name is required unless upsert=True")
		}
		out, err := buddy.CategoryCopy(src, dst, scriptOpts, buddy.CategoryCopySpec{
			SourceCategoryID: string(categoryID),
			Name:             string(name),
			Priority:         pri,
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("category_copy: cancelled")
		}
		return categoryResultDict(out.ID, string(name), pri), nil
	}
}

func categoryClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return categoryCopy(credName, logger)(thread, b, args, kwargs)
	}
}

func categoryMigrate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return categoryCopy(credName, logger)(thread, b, args, withUpsertTrue(kwargs))
	}
}

func categoryResultDict(id, name string, priority *int) starlark.Value {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("id"), starlark.String(id))
	_ = d.SetKey(starlark.String("name"), starlark.String(name))
	if priority != nil {
		_ = d.SetKey(starlark.String("priority"), starlark.MakeInt(*priority))
	}
	return resultOK(d)
}

func optionalIntPtr(v starlark.Value) (*int, error) {
	if v == nil || v == starlark.None {
		return nil, nil
	}
	n, ok := v.(starlark.Int)
	if !ok {
		return nil, fmt.Errorf("priority must be an int")
	}
	v64, _ := n.Int64()
	i := int(v64)
	return &i, nil
}

func scriptCopy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var scriptID, name, categoryID starlark.String
		upsert := starlark.Bool(false)
		var dual dualSessionKwargs
		tail := append([]any{
			"script_id", &scriptID, "name?", &name, "category_id?", &categoryID, "upsert?", &upsert,
		}, dual.unpackTail()...)
		if err := starlark.UnpackArgs("script_copy", args, kwargs, tail...); err != nil {
			return nil, err
		}
		src, dst, err := dual.refs(thread, credName)
		if err != nil {
			return resultErr(err.Error())
		}
		if bool(upsert) {
			out, err := buddy.ScriptMigrate(src, dst, scriptOpts, buddy.ScriptMigrateSpec{
				SourceScriptID: string(scriptID),
				Name:           string(name),
				CategoryID:     string(categoryID),
			}, logger)
			if err != nil {
				return resultErr(err.Error())
			}
			if out == nil {
				return resultErr("script_copy: cancelled")
			}
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(out.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(out.Name))
			_ = d.SetKey(starlark.String("created"), starlark.Bool(out.Created))
			return resultOK(d), nil
		}
		if strings.TrimSpace(string(name)) == "" {
			return resultErr("script_copy: name is required unless upsert=True")
		}
		out, err := buddy.ScriptCopy(src, dst, scriptOpts, buddy.ScriptCopySpec{
			SourceScriptID: string(scriptID),
			Name:           string(name),
			CategoryID:     string(categoryID),
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("script_copy: cancelled")
		}
		return scriptOutcomeDict(out.ID, out.Name), nil
	}
}

func scriptClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return scriptCopy(credName, logger)(thread, b, args, kwargs)
	}
}

func scriptMigrate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return scriptCopy(credName, logger)(thread, b, args, withUpsertTrue(kwargs))
	}
}

func withUpsertTrue(kwargs []starlark.Tuple) []starlark.Tuple {
	out := make([]starlark.Tuple, 0, len(kwargs)+1)
	for _, kv := range kwargs {
		if len(kv) >= 1 {
			if s, ok := starlark.AsString(kv[0]); ok && s == "upsert" {
				continue
			}
		}
		out = append(out, kv)
	}
	return append(out, starlark.Tuple{starlark.String("upsert"), starlark.True})
}

func scriptOutcomeDict(id, name string) starlark.Value {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("id"), starlark.String(id))
	_ = d.SetKey(starlark.String("name"), starlark.String(name))
	return resultOK(d)
}

func smartGroupCopy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, name, siteID starlark.String
		upsert := starlark.Bool(false)
		var dual dualSessionKwargs
		tail := append([]any{
			"group_id", &groupID, "name?", &name, "site_id?", &siteID, "upsert?", &upsert,
		}, dual.unpackTail()...)
		if err := starlark.UnpackArgs("smart_group_copy", args, kwargs, tail...); err != nil {
			return nil, err
		}
		src, dst, err := dual.refs(thread, credName)
		if err != nil {
			return resultErr(err.Error())
		}
		var sitePtr *string
		if s := strings.TrimSpace(string(siteID)); s != "" {
			sitePtr = &s
		}
		if bool(upsert) {
			out, err := buddy.SmartGroupMigrate(src, dst, scriptOpts, buddy.SmartGroupMigrateSpec{
				SourceGroupID: string(groupID),
				Name:          string(name),
				SiteID:        sitePtr,
			}, logger)
			if err != nil {
				return resultErr(err.Error())
			}
			if out == nil {
				return resultErr("smart_group_copy: cancelled")
			}
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(out.ID))
			if out.Href != "" {
				_ = d.SetKey(starlark.String("href"), starlark.String(out.Href))
			}
			_ = d.SetKey(starlark.String("name"), starlark.String(out.Name))
			_ = d.SetKey(starlark.String("created"), starlark.Bool(out.Created))
			return resultOK(d), nil
		}
		if strings.TrimSpace(string(name)) == "" {
			return resultErr("smart_group_copy: name is required unless upsert=True")
		}
		out, err := buddy.SmartGroupCopy(src, dst, scriptOpts, buddy.SmartGroupCopySpec{
			SourceGroupID: string(groupID),
			Name:          string(name),
			SiteID:        sitePtr,
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("smart_group_copy: cancelled")
		}
		return groupCreateDict(out), nil
	}
}

func smartGroupClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return smartGroupCopy(credName, logger)(thread, b, args, kwargs)
	}
}

func smartGroupMigrate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return smartGroupCopy(credName, logger)(thread, b, args, withUpsertTrue(kwargs))
	}
}

func groupCreateDict(out *buddy.GroupCreateResult) starlark.Value {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("id"), starlark.String(out.ID))
	if out.Href != "" {
		_ = d.SetKey(starlark.String("href"), starlark.String(out.Href))
	}
	return resultOK(d)
}
