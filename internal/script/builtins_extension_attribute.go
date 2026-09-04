package script

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerExtensionAttributeBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "extension_attributes", extensionAttributesBuiltin(credName, logger))
	putBuiltin(dict, "extension_attribute", extensionAttributeBuiltin(credName, logger))
	putBuiltin(dict, "extension_attribute_contents", extensionAttributeContents(credName, logger))
	putBuiltin(dict, "extension_attribute_create", extensionAttributeCreate(credName, logger))
	putBuiltin(dict, "extension_attribute_update", extensionAttributeUpdate(credName, logger))
	putBuiltin(dict, "extension_attribute_put", extensionAttributePutBuiltin(credName, logger))
	putBuiltin(dict, "extension_attribute_delete", extensionAttributeDelete(credName, logger))
	putBuiltin(dict, "extension_attribute_templates", extensionAttributeTemplatesBuiltin(credName, logger))
	putBuiltin(dict, "extension_attribute_template", extensionAttributeTemplateBuiltin(credName, logger))
	putBuiltin(dict, "extension_attribute_create_from_template", extensionAttributeCreateFromTemplate(credName, logger))
	putBuiltin(dict, "extension_attribute_history", extensionAttributeHistoryBuiltin(credName, logger))
	putBuiltin(dict, "extension_attribute_dependencies", extensionAttributeDependenciesBuiltin(credName, logger))
}

func extensionAttributesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attributes", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		results, err := buddy.ExtensionAttributeSearch(sessionPin, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, r := range results {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(r.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
			_ = d.SetKey(starlark.String("enabled"), starlark.Bool(r.Enabled))
			_ = d.SetKey(starlark.String("inputType"), starlark.String(r.InputType))
			_ = d.SetKey(starlark.String("dataType"), starlark.String(r.DataType))
			_ = d.SetKey(starlark.String("inventoryDisplayType"), starlark.String(r.InventoryDisplayType))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

func extensionAttributeBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute", args, kwargs, "id?", &idVal, "name?", &name,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		idSet := idVal != nil && idVal != starlark.None
		keySet := name != ""
		if err := xorIDOrKey("extension_attribute", idSet, keySet, "name"); err != nil {
			return nil, err
		}
		var ea *buddy.ExtensionAttributeDetail
		var err error
		if keySet {
			ea, err = buddy.ExtensionAttributeFetchByName(sessionPin, string(name), logger)
		} else {
			id, idErr := starlarkIDString(idVal, "id")
			if idErr != nil {
				return nil, idErr
			}
			ea, err = buddy.ExtensionAttributeFetchFull(sessionPin, id)
		}
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := extensionAttributeDetailValue(ea)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func extensionAttributeDetailValue(ea *buddy.ExtensionAttributeDetail) (starlark.Value, error) {
	if ea == nil {
		return nil, fmt.Errorf("extension attribute detail is nil")
	}
	b, err := json.Marshal(ea)
	if err != nil {
		return nil, err
	}
	return JSONDecodeToStarlark(string(b))
}

func extensionAttributeContents(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_contents", args, kwargs, "id", &eaID,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		contents, err := buddy.ExtensionAttributeGetContentsStr(sessionPin, string(eaID))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(contents)), nil
	}
}

func extensionAttributeCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, inputType, dataType, scriptContents, inventoryDisplay, description starlark.String
		var ldapMapping, manageExisting starlark.String
		var popupChoices starlark.Value = starlark.None
		var enabled starlark.Value = starlark.None
		var ldapAllowed starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_create", args, kwargs,
			"name", &name,
			"input_type?", &inputType,
			"data_type?", &dataType,
			"script_contents?", &scriptContents,
			"popup_menu_choices?", &popupChoices,
			"inventory_display_type?", &inventoryDisplay,
			"description?", &description,
			"enabled?", &enabled,
			"ldap_attribute_mapping?", &ldapMapping,
			"ldap_extension_attribute_allowed?", &ldapAllowed,
			"manage_existing_data?", &manageExisting,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		ea := buddy.ExtensionAttributeDetail{
			Name:                 string(name),
			InputType:            string(inputType),
			DataType:             string(dataType),
			ScriptContents:       string(scriptContents),
			InventoryDisplayType: string(inventoryDisplay),
			Description:          string(description),
			LDAPAttributeMapping: string(ldapMapping),
			ManageExistingData:    string(manageExisting),
		}
		if popupChoices != nil && popupChoices != starlark.None {
			g, err := StarlarkValueToGo(popupChoices)
			if err != nil {
				return resultErr(err.Error())
			}
			ea.PopupMenuChoices = stringSliceFromGo(g)
		}
		if enabled != nil && enabled != starlark.None {
			b, ok := enabled.(starlark.Bool)
			if !ok {
				return resultErr("enabled must be a bool")
			}
			v := bool(b)
			ea.Enabled = &v
		}
		if ldapAllowed != nil && ldapAllowed != starlark.None {
			b, ok := ldapAllowed.(starlark.Bool)
			if !ok {
				return resultErr("ldap_extension_attribute_allowed must be a bool")
			}
			v := bool(b)
			ea.LDAPExtensionAttributeAllowed = &v
		}
		created, err := buddy.ExtensionAttributePostDetail(sessionPin, scriptOpts, ea, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if created == nil {
			return resultErr("extension_attribute_create: cancelled")
		}
		val, err := extensionAttributeDetailValue(created)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func stringSliceFromGo(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

func extensionAttributeUpdate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID, file starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_update", args, kwargs, "id", &eaID, "file", &file,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		if err := buddy.ExtensionAttributeUpdate(sessionPin, scriptOpts, string(eaID), string(file), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func extensionAttributePutBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID starlark.String
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_put", args, kwargs, "id", &eaID, "dict", &bodyVal,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		g, err := StarlarkValueToGo(bodyVal)
		if err != nil {
			return resultErr(err.Error())
		}
		m, ok := g.(map[string]interface{})
		if !ok {
			return resultErr("dict must be a map/dict")
		}
		updated, err := buddy.ExtensionAttributePutFields(sessionPin, scriptOpts, string(eaID), m, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if updated == nil {
			return resultErr("extension_attribute_put: cancelled")
		}
		val, err := extensionAttributeDetailValue(updated)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func extensionAttributeDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var idsVal starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_delete", args, kwargs, "id?", &idVal, "ids?", &idsVal,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		idsSet := idsVal != nil && idsVal != starlark.None
		if err := xorIDOrKey("extension_attribute_delete", idSet, idsSet, "ids"); err != nil {
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
			return nil, fmt.Errorf("extension_attribute_delete: %w", err)
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		if err := buddy.ExtensionAttributeDelete(sessionPin, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func extensionAttributeTemplatesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_templates", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		results, err := buddy.ExtensionAttributeTemplateSearch(sessionPin, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, r := range results {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("templateId"), starlark.String(r.TemplateID))
			_ = d.SetKey(starlark.String("templateName"), starlark.String(r.TemplateName))
			_ = d.SetKey(starlark.String("templateCategoryName"), starlark.String(r.TemplateCategoryName))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

func extensionAttributeTemplateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var tmplID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_template", args, kwargs, "id", &tmplID,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		ea, err := buddy.ExtensionAttributeFetchTemplate(sessionPin, string(tmplID))
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := extensionAttributeDetailValue(ea)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func extensionAttributeCreateFromTemplate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var tmplID, name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_create_from_template", args, kwargs, "id", &tmplID, "name?", &name,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		created, err := buddy.ExtensionAttributeCreateFromTemplate(sessionPin, scriptOpts, string(tmplID), string(name), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if created == nil {
			return resultErr("extension_attribute_create_from_template: cancelled")
		}
		val, err := extensionAttributeDetailValue(created)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func extensionAttributeHistoryBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_history", args, kwargs, "id", &eaID,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		entries, _, err := buddy.ExtensionAttributeFetchHistory(sessionPin, string(eaID), 20)
		if err != nil {
			return resultErr(err.Error())
		}
		b, err := json.Marshal(entries)
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

func extensionAttributeDependenciesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("extension_attribute_dependencies", args, kwargs, "id", &eaID,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		deps, err := buddy.ExtensionAttributeFetchDependencies(sessionPin, string(eaID))
		if err != nil {
			return resultErr(err.Error())
		}
		b, err := json.Marshal(deps)
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

func extensionAttributeCopy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID, name starlark.String
		upsert := starlark.Bool(false)
		var dual dualSessionKwargs
		tail := append([]any{
			"id", &eaID, "name?", &name, "upsert?", &upsert,
		}, dual.unpackTail()...)
		if err := starlark.UnpackArgs("extension_attribute_copy", args, kwargs, tail...); err != nil {
			return nil, err
		}
		src, dst, err := dual.refs(thread, credName)
		if err != nil {
			return resultErr(err.Error())
		}
		if bool(upsert) {
			out, err := buddy.ExtensionAttributeMigrate(src, dst, scriptOpts, buddy.ExtensionAttributeMigrateSpec{
				SourceID: string(eaID),
				Name:     string(name),
			}, logger)
			if err != nil {
				return resultErr(err.Error())
			}
			if out == nil {
				return resultErr("extension_attribute_copy: cancelled")
			}
			val, err := extensionAttributeDetailValue(&out.EA)
			if err != nil {
				return resultErr(err.Error())
			}
			d, ok := val.(*starlark.Dict)
			if !ok {
				return resultErr("extension_attribute_copy: expected dict")
			}
			_ = d.SetKey(starlark.String("created"), starlark.Bool(out.Created))
			return resultOK(d), nil
		}
		if strings.TrimSpace(string(name)) == "" {
			return resultErr("extension_attribute_copy: name is required unless upsert=True")
		}
		out, err := buddy.ExtensionAttributeCopy(src, dst, scriptOpts, buddy.ExtensionAttributeCopySpec{
			SourceID: string(eaID),
			Name:     string(name),
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("extension_attribute_copy: cancelled")
		}
		val, err := extensionAttributeDetailValue(&out.EA)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func extensionAttributeClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return extensionAttributeCopy(credName, logger)(thread, b, args, kwargs)
	}
}

func extensionAttributeMigrate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return extensionAttributeCopy(credName, logger)(thread, b, args, withUpsertTrue(kwargs))
	}
}
