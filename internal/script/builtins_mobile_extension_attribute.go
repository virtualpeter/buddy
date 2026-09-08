package script

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerMobileExtensionAttributeBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "mobile_extension_attributes", mobileExtensionAttributesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_extension_attribute", mobileExtensionAttributeBuiltin(credName, logger))
	putBuiltin(dict, "mobile_extension_attribute_create", mobileExtensionAttributeCreate(credName, logger))
	putBuiltin(dict, "mobile_extension_attribute_put", mobileExtensionAttributePutBuiltin(credName, logger))
	putBuiltin(dict, "mobile_extension_attribute_delete", mobileExtensionAttributeDelete(credName, logger))
	putBuiltin(dict, "mobile_extension_attribute_copy", mobileExtensionAttributeCopy(credName, logger))
	putBuiltin(dict, "mobile_extension_attribute_clone", mobileExtensionAttributeClone(credName, logger))
	putBuiltin(dict, "mobile_extension_attribute_migrate", mobileExtensionAttributeMigrate(credName, logger))
}

func mobileExtensionAttributesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_extension_attributes", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
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
		results, err := buddy.MobileExtensionAttributeSearch(sessionPin, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, r := range results {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.String(r.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
			_ = d.SetKey(starlark.String("inputType"), starlark.String(r.InputType))
			_ = d.SetKey(starlark.String("dataType"), starlark.String(r.DataType))
			_ = d.SetKey(starlark.String("inventoryDisplayType"), starlark.String(r.InventoryDisplayType))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

func mobileExtensionAttributeDetailValue(ea *buddy.MobileExtensionAttributeDetail) (starlark.Value, error) {
	if ea == nil {
		return nil, fmt.Errorf("mobile extension attribute detail is nil")
	}
	b, err := json.Marshal(ea)
	if err != nil {
		return nil, err
	}
	return JSONDecodeToStarlark(string(b))
}

func mobileExtensionAttributeBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_extension_attribute", args, kwargs, "id?", &idVal, "name?", &name,
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
		if err := xorIDOrKey("mobile_extension_attribute", idSet, keySet, "name"); err != nil {
			return nil, err
		}
		var ea *buddy.MobileExtensionAttributeDetail
		var err error
		if keySet {
			ea, err = buddy.MobileExtensionAttributeFetchByName(sessionPin, string(name), logger)
		} else {
			id, idErr := starlarkIDString(idVal, "id")
			if idErr != nil {
				return nil, idErr
			}
			ea, err = buddy.MobileExtensionAttributeFetchFull(sessionPin, id)
		}
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := mobileExtensionAttributeDetailValue(ea)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func mobileExtensionAttributeCreate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, inputType, dataType, inventoryDisplay, description, ldapMapping starlark.String
		var popupChoices starlark.Value = starlark.None
		var ldapAllowed starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_extension_attribute_create", args, kwargs,
			"name", &name,
			"input_type?", &inputType,
			"data_type?", &dataType,
			"popup_menu_choices?", &popupChoices,
			"inventory_display_type?", &inventoryDisplay,
			"description?", &description,
			"ldap_attribute_mapping?", &ldapMapping,
			"ldap_extension_attribute_allowed?", &ldapAllowed,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		ea := buddy.MobileExtensionAttributeDetail{
			Name:                 string(name),
			InputType:            string(inputType),
			DataType:             string(dataType),
			InventoryDisplayType: string(inventoryDisplay),
			Description:          string(description),
			LDAPAttributeMapping: string(ldapMapping),
		}
		if popupChoices != nil && popupChoices != starlark.None {
			g, err := StarlarkValueToGo(popupChoices)
			if err != nil {
				return resultErr(err.Error())
			}
			ea.PopupMenuChoices = stringSliceFromGo(g)
		}
		if ldapAllowed != nil && ldapAllowed != starlark.None {
			b, ok := ldapAllowed.(starlark.Bool)
			if !ok {
				return resultErr("ldap_extension_attribute_allowed must be a bool")
			}
			v := bool(b)
			ea.LDAPExtensionAttributeAllowed = &v
		}
		created, err := buddy.MobileExtensionAttributePostDetail(sessionPin, scriptOpts, ea, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if created == nil {
			return resultErr("mobile_extension_attribute_create: cancelled")
		}
		val, err := mobileExtensionAttributeDetailValue(created)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func mobileExtensionAttributePutBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID starlark.String
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_extension_attribute_put", args, kwargs, "id", &eaID, "dict", &bodyVal,
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
		updated, err := buddy.MobileExtensionAttributePutFields(sessionPin, scriptOpts, string(eaID), m, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if updated == nil {
			return resultErr("mobile_extension_attribute_put: cancelled")
		}
		val, err := mobileExtensionAttributeDetailValue(updated)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func mobileExtensionAttributeDelete(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var idsVal starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_extension_attribute_delete", args, kwargs, "id?", &idVal, "ids?", &idsVal,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		idSet := idVal != nil && idVal != starlark.None
		idsSet := idsVal != nil && idsVal != starlark.None
		if err := xorIDOrKey("mobile_extension_attribute_delete", idSet, idsSet, "ids"); err != nil {
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
			return nil, fmt.Errorf("mobile_extension_attribute_delete: %w", err)
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		if err := buddy.MobileExtensionAttributeDelete(sessionPin, scriptOpts, ids, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func mobileExtensionAttributeCopy(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var eaID, name starlark.String
		upsert := starlark.Bool(false)
		var dual dualSessionKwargs
		tail := append([]any{
			"id", &eaID, "name?", &name, "upsert?", &upsert,
		}, dual.unpackTail()...)
		if err := starlark.UnpackArgs("mobile_extension_attribute_copy", args, kwargs, tail...); err != nil {
			return nil, err
		}
		src, dst, err := dual.refs(thread, credName)
		if err != nil {
			return resultErr(err.Error())
		}
		if bool(upsert) {
			out, err := buddy.MobileExtensionAttributeMigrate(src, dst, scriptOpts, buddy.MobileExtensionAttributeMigrateSpec{
				SourceID: string(eaID),
				Name:     string(name),
			}, logger)
			if err != nil {
				return resultErr(err.Error())
			}
			if out == nil {
				return resultErr("mobile_extension_attribute_copy: cancelled")
			}
			val, err := mobileExtensionAttributeDetailValue(&out.EA)
			if err != nil {
				return resultErr(err.Error())
			}
			d, ok := val.(*starlark.Dict)
			if !ok {
				return resultErr("mobile_extension_attribute_copy: expected dict")
			}
			_ = d.SetKey(starlark.String("created"), starlark.Bool(out.Created))
			return resultOK(d), nil
		}
		if strings.TrimSpace(string(name)) == "" {
			return resultErr("mobile_extension_attribute_copy: name is required unless upsert=True")
		}
		out, err := buddy.MobileExtensionAttributeCopy(src, dst, scriptOpts, buddy.MobileExtensionAttributeCopySpec{
			SourceID: string(eaID),
			Name:     string(name),
		}, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("mobile_extension_attribute_copy: cancelled")
		}
		val, err := mobileExtensionAttributeDetailValue(&out.EA)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func mobileExtensionAttributeClone(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return mobileExtensionAttributeCopy(credName, logger)(thread, b, args, kwargs)
	}
}

func mobileExtensionAttributeMigrate(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return mobileExtensionAttributeCopy(credName, logger)(thread, b, args, withUpsertTrue(kwargs))
	}
}
