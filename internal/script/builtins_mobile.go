package script

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

// registerMobileBuiltins registers the Jamf Pro mobile-device family:
// inventory (mobile_devices / mobile_device), MDM actions, smart/static
// mobile-device groups, and mobile configuration profiles (Classic XML).
func registerMobileBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "mobile_devices", mobileDevicesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_device", mobileDeviceBuiltin(credName, logger))
	putBuiltin(dict, "mobile_device_send_mdm_command", mobileDeviceSendMDMCommandBuiltin(credName, logger))
	putBuiltin(dict, "mobile_device_update_inventory", mobileDeviceUpdateInventoryBuiltin(credName, logger))
	putBuiltin(dict, "mobile_device_restart", mobileDeviceRestartBuiltin(credName, logger))
	putBuiltin(dict, "mobile_device_lock", mobileDeviceLockBuiltin(credName, logger))
	putBuiltin(dict, "mobile_device_erase", mobileDeviceEraseBuiltin(credName, logger))

	putBuiltin(dict, "mobile_smart_groups", mobileSmartGroupsBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_groups", mobileStaticGroupsBuiltin(credName, logger))
	putBuiltin(dict, "mobile_smart_group", mobileSmartGroupBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_group", mobileStaticGroupBuiltin(credName, logger))
	putBuiltin(dict, "mobile_smart_group_create", mobileSmartGroupCreateBuiltin(credName, logger))
	putBuiltin(dict, "mobile_smart_group_put", mobileSmartGroupPutBuiltin(credName, logger))
	putBuiltin(dict, "mobile_smart_group_delete", mobileSmartGroupDeleteBuiltin(credName, logger))
	putBuiltin(dict, "mobile_smart_group_criteria", mobileSmartGroupCriteriaBuiltin(credName, logger))
	putBuiltin(dict, "mobile_smart_group_set_criteria", mobileSmartGroupSetCriteriaBuiltin(credName, logger))
	putBuiltin(dict, "mobile_smart_group_devices", mobileSmartGroupDevicesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_group_create", mobileStaticGroupCreateBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_group_delete", mobileStaticGroupDeleteBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_group_devices", mobileStaticGroupDevicesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_group_add_devices", mobileStaticGroupAddDevicesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_group_remove_devices", mobileStaticGroupRemoveDevicesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_static_group_set_devices", mobileStaticGroupSetDevicesBuiltin(credName, logger))

	putBuiltin(dict, "mobile_profiles", mobileProfilesBuiltin(credName, logger))
	putBuiltin(dict, "mobile_profile_xml", mobileProfileXMLBuiltin(credName, logger))
	putBuiltin(dict, "mobile_profile_put_xml", mobileProfilePutXMLBuiltin(credName, logger))
	putBuiltin(dict, "mobile_profile_create_xml", mobileProfileCreateXMLBuiltin(credName, logger))
	putBuiltin(dict, "mobile_profile_delete", mobileProfileDeleteBuiltin(credName, logger))
}

func mobileDeviceRecordDict(r buddy.MobileDeviceRecord) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("id"), starlark.String(r.ID))
	_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
	_ = d.SetKey(starlark.String("serial"), starlark.String(r.SerialNumber))
	_ = d.SetKey(starlark.String("udid"), starlark.String(r.UDID))
	_ = d.SetKey(starlark.String("model"), starlark.String(r.Model))
	_ = d.SetKey(starlark.String("model_identifier"), starlark.String(r.ModelIdentifier))
	_ = d.SetKey(starlark.String("username"), starlark.String(r.Username))
	_ = d.SetKey(starlark.String("type"), starlark.String(r.Type))
	_ = d.SetKey(starlark.String("management_id"), starlark.String(r.ManagementID))
	_ = d.SetKey(starlark.String("wifi_mac_address"), starlark.String(r.WifiMacAddress))
	_ = d.SetKey(starlark.String("phone_number"), starlark.String(r.PhoneNumber))
	return d
}

// parseMobileDeviceIDInts converts a Starlark list of int or str to []int.
func parseMobileDeviceIDInts(list *starlark.List, fn string) ([]int, error) {
	var ids []int
	for i := 0; i < list.Len(); i++ {
		switch t := list.Index(i).(type) {
		case starlark.Int:
			n, _ := t.Int64()
			ids = append(ids, int(n))
		case starlark.String:
			n, err := strconv.Atoi(strings.TrimSpace(string(t)))
			if err != nil {
				return nil, fmt.Errorf("%s: ids must be ints or numeric strings", fn)
			}
			ids = append(ids, n)
		default:
			return nil, fmt.Errorf("%s: ids must be a list of int or str", fn)
		}
	}
	return ids, nil
}

// parseMobileDeviceIDStrings converts a Starlark list of int or str to []string.
func parseMobileDeviceIDStrings(list *starlark.List, fn string) ([]string, error) {
	var ids []string
	for i := 0; i < list.Len(); i++ {
		switch t := list.Index(i).(type) {
		case starlark.Int:
			ids = append(ids, t.String())
		case starlark.String:
			s := strings.TrimSpace(string(t))
			if s == "" {
				return nil, fmt.Errorf("%s: device_ids must not contain empty strings", fn)
			}
			ids = append(ids, s)
		default:
			return nil, fmt.Errorf("%s: device_ids must be a list of int or str", fn)
		}
	}
	return ids, nil
}

func mobileGroupSummaryList(groups []buddy.MobileGroupSummary) *starlark.List {
	list := starlark.NewList(nil)
	for _, g := range groups {
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(g.GroupID))
		_ = d.SetKey(starlark.String("name"), starlark.String(g.GroupName))
		_ = d.SetKey(starlark.String("description"), starlark.String(g.GroupDescription))
		_ = d.SetKey(starlark.String("count"), starlark.MakeInt(g.Count))
		_ = list.Append(d)
	}
	return list
}

func mobileMemberList(members []buddy.MobileDeviceMember) *starlark.List {
	list := starlark.NewList(nil)
	for _, m := range members {
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(m.ID))
		_ = d.SetKey(starlark.String("name"), starlark.String(m.Name))
		_ = d.SetKey(starlark.String("serial"), starlark.String(m.Serial))
		_ = list.Append(d)
	}
	return list
}

// mobile_devices(filter?, page?, page_size?, tenant?, surface?, auth?) → list of summary dicts
// filter is client-side (GET /api/v2/mobile-devices has no server-side filter).
func mobileDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_devices", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
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
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		records, err := buddy.MobileDeviceSearch(sessionPin, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, r := range records {
			_ = list.Append(mobileDeviceRecordDict(r))
		}
		return resultOK(list), nil
	}
}

// mobile_device(id=?, name=?, serial=?, udid=?, detail?, tenant?, surface?, auth?) → dict
// Pass exactly one of id/name/serial/udid. detail=True fetches /detail (full inventory).
func mobileDeviceBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name, serial, udid starlark.String
		detail := starlark.Bool(false)
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_device", args, kwargs,
			"id?", &idVal, "name?", &name, "serial?", &serial, "udid?", &udid, "detail?", &detail,
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
		idStr := ""
		if idVal != starlark.None {
			s, err := starlarkIDString(idVal, "id")
			if err != nil {
				return nil, err
			}
			idStr = s
		}
		rec, err := buddy.MobileDeviceResolve(c, idStr, string(name), string(serial), string(udid), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		var body []byte
		if bool(detail) {
			body, err = buddy.MobileDeviceFetchDetail(c, rec.ID)
		} else {
			body, err = buddy.MobileDeviceFetchInventory(c, rec.ID)
		}
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

// mobileMDMActionBuiltin handles the shared unpack/session/id-parse plumbing for
// mobile MDM actions that take (ids, tenant?, surface?, auth?).
func mobileMDMActionBuiltin(fn, credName string, run func(c string, ids []int) error) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs(fn, args, kwargs, "ids", &deviceIDs,
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
		ids, err := parseMobileDeviceIDInts(deviceIDs, fn)
		if err != nil {
			return nil, err
		}
		if err := run(sessionPin, ids); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobile_device_send_mdm_command(ids, command_type, command_data?, tenant?, surface?, auth?) → None
func mobileDeviceSendMDMCommandBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceIDs *starlark.List
		var commandType starlark.String
		var commandDataVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_device_send_mdm_command", args, kwargs,
			"ids", &deviceIDs,
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
		ids, err := parseMobileDeviceIDInts(deviceIDs, "mobile_device_send_mdm_command")
		if err != nil {
			return nil, err
		}
		var commandData map[string]interface{}
		if commandDataVal != nil && commandDataVal != starlark.None {
			g, err := StarlarkValueToGo(commandDataVal)
			if err != nil {
				return resultErr(err.Error())
			}
			m, ok := g.(map[string]interface{})
			if !ok {
				return resultErr("command_data must be a dict")
			}
			commandData = m
		}
		if err := buddy.MobileDeviceSendMDMCommand(sessionPin, ids, string(commandType), commandData, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobile_device_update_inventory(ids, tenant?, surface?, auth?) → None
func mobileDeviceUpdateInventoryBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileMDMActionBuiltin("mobile_device_update_inventory", credName, func(c string, ids []int) error {
		return buddy.MobileDeviceUpdateInventory(c, scriptOpts, ids, logger)
	})
}

// mobile_device_restart(ids, tenant?, surface?, auth?) → None
func mobileDeviceRestartBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileMDMActionBuiltin("mobile_device_restart", credName, func(c string, ids []int) error {
		return buddy.MobileDeviceRestart(c, scriptOpts, ids, logger)
	})
}

// mobile_device_lock(ids, message?, phone?, pin?, tenant?, surface?, auth?) → None
func mobileDeviceLockBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceIDs *starlark.List
		var message, phone, pin starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_device_lock", args, kwargs,
			"ids", &deviceIDs, "message?", &message, "phone?", &phone, "pin?", &pin,
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
		ids, err := parseMobileDeviceIDInts(deviceIDs, "mobile_device_lock")
		if err != nil {
			return nil, err
		}
		if err := buddy.MobileDeviceLock(sessionPin, scriptOpts, ids, string(message), string(phone), string(pin), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobile_device_erase(ids, pin?, obliteration?, preserve_data_plan?, disallow_proximity_setup?, tenant?, surface?, auth?) → None
func mobileDeviceEraseBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceIDs *starlark.List
		var pin, obliteration starlark.String
		preserveDataPlan := starlark.Bool(false)
		disallowProximitySetup := starlark.Bool(false)
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_device_erase", args, kwargs,
			"ids", &deviceIDs, "pin?", &pin, "obliteration?", &obliteration,
			"preserve_data_plan?", &preserveDataPlan, "disallow_proximity_setup?", &disallowProximitySetup,
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
		ids, err := parseMobileDeviceIDInts(deviceIDs, "mobile_device_erase")
		if err != nil {
			return nil, err
		}
		if err := buddy.MobileDeviceErase(sessionPin, scriptOpts, ids, string(pin), string(obliteration), bool(preserveDataPlan), bool(disallowProximitySetup), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobileGroupsListBuiltin is the shared list builtin for mobile smart/static groups.
func mobileGroupsListBuiltin(fn, credName string, search func(c, filter string, opts buddy.PageOpts, logger *slog.Logger) ([]buddy.MobileGroupSummary, error), logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs(fn, args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
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
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		groups, err := search(sessionPin, string(filter), opts, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(mobileGroupSummaryList(groups)), nil
	}
}

// mobile_smart_groups(filter?, page?, page_size?, tenant?, surface?, auth?) → list of {id, name, description, count}
func mobileSmartGroupsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupsListBuiltin("mobile_smart_groups", credName, buddy.MobileGroupSearchSmart, logger)
}

// mobile_static_groups(filter?, page?, page_size?, tenant?, surface?, auth?) → list of {id, name, description, count}
func mobileStaticGroupsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupsListBuiltin("mobile_static_groups", credName, buddy.MobileGroupSearchStatic, logger)
}

// mobileGroupResolveID resolves id= or name= (exact, case-insensitive) to a group ID.
func mobileGroupResolveID(c, id, name string, search func(c, filter string, opts buddy.PageOpts, logger *slog.Logger) ([]buddy.MobileGroupSummary, error), logger *slog.Logger) (string, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if (id == "") == (name == "") {
		return "", fmt.Errorf("pass either id= or name=, not both")
	}
	if id != "" {
		return id, nil
	}
	groups, err := search(c, name, buddy.PageOpts{Page: buddy.PageAll}, logger)
	if err != nil {
		return "", err
	}
	for _, g := range groups {
		if strings.EqualFold(strings.TrimSpace(g.GroupName), name) {
			return g.GroupID, nil
		}
	}
	return "", fmt.Errorf("no mobile-device group with name %q", name)
}

// mobileGroupLoadBuiltin is the shared load-one builtin for mobile smart/static groups.
func mobileGroupLoadBuiltin(fn, credName string, search func(c, filter string, opts buddy.PageOpts, logger *slog.Logger) ([]buddy.MobileGroupSummary, error), fetch func(c, id string) ([]byte, error), logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs(fn, args, kwargs, "id?", &idVal, "name?", &name,
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
		idStr := ""
		if idVal != starlark.None {
			s, err := starlarkIDString(idVal, "id")
			if err != nil {
				return nil, err
			}
			idStr = s
		}
		groupID, err := mobileGroupResolveID(c, idStr, string(name), search, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		body, err := fetch(c, groupID)
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := JSONDecodeToStarlark(string(body))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

// mobile_smart_group(id=?) | mobile_smart_group(name=?, tenant?, surface?, auth?) → dict
func mobileSmartGroupBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupLoadBuiltin("mobile_smart_group", credName, buddy.MobileGroupSearchSmart, buddy.MobileGroupFetchSmartRaw, logger)
}

// mobile_static_group(id=?) | mobile_static_group(name=?, tenant?, surface?, auth?) → dict
func mobileStaticGroupBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupLoadBuiltin("mobile_static_group", credName, buddy.MobileGroupSearchStatic, buddy.MobileGroupFetchStaticRaw, logger)
}

// mobileGroupCreateBuiltin is the shared create builtin (body dict → {id, href?}).
func mobileGroupCreateBuiltin(fn, credName string, create func(c string, opts buddy.Opts, payload []byte, logger *slog.Logger) (*buddy.GroupCreateResult, error), logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs(fn, args, kwargs, "body", &bodyVal,
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
		jsonStr, err := StarlarkToJSON(bodyVal)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body JSON: %v", err))
		}
		res, err := create(sessionPin, scriptOpts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.String(res.ID))
		if res.Href != "" {
			_ = d.SetKey(starlark.String("href"), starlark.String(res.Href))
		}
		return resultOK(d), nil
	}
}

// mobile_smart_group_create(body, tenant?, surface?, auth?) → dict {"id", "href"?}
func mobileSmartGroupCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupCreateBuiltin("mobile_smart_group_create", credName, buddy.MobileGroupCreateSmart, logger)
}

// mobile_static_group_create(body, tenant?, surface?, auth?) → dict {"id", "href"?}
func mobileStaticGroupCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupCreateBuiltin("mobile_static_group_create", credName, buddy.MobileGroupCreateStatic, logger)
}

// mobile_smart_group_put(id, body, tenant?, surface?, auth?) → None
func mobileSmartGroupPutBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var bodyVal starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_smart_group_put", args, kwargs, "id", &groupID, "body", &bodyVal,
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
		jsonStr, err := StarlarkToJSON(bodyVal)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body JSON: %v", err))
		}
		if err := buddy.MobileGroupPutSmartRaw(sessionPin, scriptOpts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobileGroupIDActionBuiltin is the shared (id, session) → None builtin.
func mobileGroupIDActionBuiltin(fn, credName string, run func(c, id string) error) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs(fn, args, kwargs, "id", &groupID,
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
		if err := run(sessionPin, string(groupID)); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobile_smart_group_delete(id, tenant?, surface?, auth?) → None
func mobileSmartGroupDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupIDActionBuiltin("mobile_smart_group_delete", credName, func(c, id string) error {
		return buddy.MobileGroupDeleteSmart(c, scriptOpts, id, logger)
	})
}

// mobile_static_group_delete(id, tenant?, surface?, auth?) → None
func mobileStaticGroupDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupIDActionBuiltin("mobile_static_group_delete", credName, func(c, id string) error {
		return buddy.MobileGroupDeleteStatic(c, scriptOpts, id, logger)
	})
}

// mobile_smart_group_criteria(id, filter?, tenant?, surface?, auth?) → list[dict]
// filter? is a case-insensitive substring matched against each criterion's name.
func mobileSmartGroupCriteriaBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_smart_group_criteria", args, kwargs, "id", &groupID, "filter?", &filter,
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
		criteria, err := buddy.MobileSmartGroupCriteriaValue(sessionPin, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		if f := strings.ToLower(strings.TrimSpace(string(filter))); f != "" {
			var filtered []any
			for _, item := range criteria {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				name, _ := m["name"].(string)
				if strings.Contains(strings.ToLower(name), f) {
					filtered = append(filtered, item)
				}
			}
			criteria = filtered
		}
		val, err := GoValueToStarlark(criteria)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

// mobile_smart_group_set_criteria(id, criteria, tenant?, surface?, auth?) → None
func mobileSmartGroupSetCriteriaBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var criteria *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_smart_group_set_criteria", args, kwargs, "id", &groupID, "criteria", &criteria,
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
		jsonStr, err := StarlarkToJSON(criteria)
		if err != nil {
			return resultErr(fmt.Sprintf("encode criteria JSON: %v", err))
		}
		if err := buddy.MobileSmartGroupSetCriteria(sessionPin, scriptOpts, string(groupID), []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobileGroupMembersBuiltin is the shared members-list builtin.
func mobileGroupMembersBuiltin(fn, credName string, fetch func(c, id string) ([]buddy.MobileDeviceMember, error)) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs(fn, args, kwargs, "id", &groupID, "filter?", &filter,
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
		members, err := fetch(sessionPin, string(groupID))
		if err != nil {
			return resultErr(err.Error())
		}
		members = buddy.FilterMobileGroupMembers(members, string(filter))
		return resultOK(mobileMemberList(members)), nil
	}
}

// mobile_smart_group_devices(id, filter?, tenant?, surface?, auth?) → list of {id, name, serial}
func mobileSmartGroupDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupMembersBuiltin("mobile_smart_group_devices", credName, buddy.MobileSmartGroupFetchMembers)
}

// mobile_static_group_devices(id, filter?, tenant?, surface?, auth?) → list of {id, name, serial}
func mobileStaticGroupDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileGroupMembersBuiltin("mobile_static_group_devices", credName, buddy.MobileStaticGroupFetchMembers)
}

// mobileStaticGroupMembershipBuiltin is the shared (id, device_ids) → None mutate builtin.
func mobileStaticGroupMembershipBuiltin(fn, credName string, run func(c, id string, deviceIDs []string) error) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var deviceIDs *starlark.List
		var sess sessionArgs
		if err := starlark.UnpackArgs(fn, args, kwargs, "id", &groupID, "device_ids", &deviceIDs,
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
		ids, err := parseMobileDeviceIDStrings(deviceIDs, fn)
		if err != nil {
			return nil, err
		}
		if err := run(sessionPin, string(groupID), ids); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobile_static_group_add_devices(id, device_ids, tenant?, surface?, auth?) → None
func mobileStaticGroupAddDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileStaticGroupMembershipBuiltin("mobile_static_group_add_devices", credName, func(c, id string, deviceIDs []string) error {
		return buddy.MobileStaticGroupAssign(c, scriptOpts, id, deviceIDs, nil, logger)
	})
}

// mobile_static_group_remove_devices(id, device_ids, tenant?, surface?, auth?) → None
func mobileStaticGroupRemoveDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileStaticGroupMembershipBuiltin("mobile_static_group_remove_devices", credName, func(c, id string, deviceIDs []string) error {
		return buddy.MobileStaticGroupAssign(c, scriptOpts, id, nil, deviceIDs, logger)
	})
}

// mobile_static_group_set_devices(id, device_ids, tenant?, surface?, auth?) → None
func mobileStaticGroupSetDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return mobileStaticGroupMembershipBuiltin("mobile_static_group_set_devices", credName, func(c, id string, deviceIDs []string) error {
		return buddy.MobileStaticGroupSetDevices(c, scriptOpts, id, deviceIDs, logger)
	})
}

// mobile_profiles(filter?, tenant?, surface?, auth?) → list of {id, name}
func mobileProfilesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_profiles", args, kwargs, "filter?", &filter,
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
		records, err := buddy.MobileProfileSearch(sessionPin, string(filter), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, r := range records {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.MakeInt(r.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(r.Name))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

// mobileProfileResolveID resolves id= or name= (exact, then unique substring) to a profile ID.
func mobileProfileResolveID(c, idStr, name string, logger *slog.Logger) (int, error) {
	idStr = strings.TrimSpace(idStr)
	name = strings.TrimSpace(name)
	if (idStr == "") == (name == "") {
		return 0, fmt.Errorf("pass either id= or name=, not both")
	}
	if idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			return 0, fmt.Errorf("invalid mobile-profile id %q", idStr)
		}
		return id, nil
	}
	records, err := buddy.MobileProfileSearch(c, name, logger)
	if err != nil {
		return 0, err
	}
	for _, r := range records {
		if strings.EqualFold(strings.TrimSpace(r.Name), name) {
			return r.ID, nil
		}
	}
	switch len(records) {
	case 1:
		return records[0].ID, nil
	case 0:
		return 0, fmt.Errorf("no mobile profile with name %q", name)
	default:
		return 0, fmt.Errorf("%d mobile profiles match name %q; use id=", len(records), name)
	}
}

// mobile_profile_xml(id=?) | mobile_profile_xml(name=?, tenant?, surface?, auth?) → str (Classic XML)
func mobileProfileXMLBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_profile_xml", args, kwargs, "id?", &idVal, "name?", &name,
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
		idStr := ""
		if idVal != starlark.None {
			s, err := starlarkIDString(idVal, "id")
			if err != nil {
				return nil, err
			}
			idStr = s
		}
		id, err := mobileProfileResolveID(c, idStr, string(name), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		body, err := buddy.MobileProfileFetchXML(c, id)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(body)), nil
	}
}

// mobile_profile_put_xml(id, xml_str, redeploy?, tenant?, surface?, auth?) → None
func mobileProfilePutXMLBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var xmlStr starlark.String
		redeploy := starlark.Bool(false)
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_profile_put_xml", args, kwargs,
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
		pid, _ := profileID.Int64()
		if err := buddy.MobileProfilePutXML(sessionPin, scriptOpts, int(pid), []byte(xmlStr), bool(redeploy), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

// mobile_profile_create_xml(xml_str, tenant?, surface?, auth?) → dict {"id"}
func mobileProfileCreateXMLBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var xmlStr starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_profile_create_xml", args, kwargs, "xml_str", &xmlStr,
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
		id, err := buddy.MobileProfileCreateXML(sessionPin, scriptOpts, []byte(xmlStr), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("id"), starlark.MakeInt(id))
		return resultOK(d), nil
	}
}

// mobile_profile_delete(id, tenant?, surface?, auth?) → None
func mobileProfileDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var profileID starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("mobile_profile_delete", args, kwargs, "id", &profileID,
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
		if err := buddy.MobileProfileDelete(sessionPin, scriptOpts, int(pid), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}
