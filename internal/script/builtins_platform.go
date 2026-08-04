package script

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerPlatformBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "devices", devicesBuiltin(credName, logger))
	putBuiltin(dict, "device", deviceBuiltin(credName, logger))
	putBuiltin(dict, "device_set", deviceSetBuiltin(credName, logger))
	putBuiltin(dict, "device_delete", deviceDeleteBuiltin(credName, logger))
	putBuiltin(dict, "device_applications", deviceApplicationsBuiltin(credName, logger))
	putBuiltin(dict, "device_check_in", deviceActionBuiltin(credName, logger, "check_in", buddy.DeviceCheckIn))
	putBuiltin(dict, "device_restart", deviceActionBuiltin(credName, logger, "restart", buddy.DeviceRestart))
	putBuiltin(dict, "device_shutdown", deviceActionBuiltin(credName, logger, "shutdown", buddy.DeviceShutdown))
	putBuiltin(dict, "device_erase", deviceActionBuiltin(credName, logger, "erase", buddy.DeviceErase))
	putBuiltin(dict, "device_unmanage", deviceActionBuiltin(credName, logger, "unmanage", buddy.DeviceUnmanage))

	putBuiltin(dict, "device_groups", deviceGroupsBuiltin(credName, logger))
	putBuiltin(dict, "device_group", deviceGroupBuiltin(credName, logger))
	putBuiltin(dict, "device_group_create", deviceGroupCreateBuiltin(credName, logger))
	putBuiltin(dict, "device_group_set", deviceGroupSetBuiltin(credName, logger))
	putBuiltin(dict, "device_group_delete", deviceGroupDeleteBuiltin(credName, logger))
	putBuiltin(dict, "device_group_members", deviceGroupMembersBuiltin(credName, logger))
	putBuiltin(dict, "device_group_set_members", deviceGroupSetMembersBuiltin(credName, logger))

	putBuiltin(dict, "ddm_declaration_devices", ddmDeclarationDevicesBuiltin(credName, logger))
	putBuiltin(dict, "ddm_device_declarations", ddmDeviceDeclarationsBuiltin(credName, logger))
	putBuiltin(dict, "ddm_device_channels", ddmDeviceChannelsBuiltin(credName, logger))

	putBuiltin(dict, "benchmarks", benchmarksBuiltin(credName, logger))
	putBuiltin(dict, "benchmark", benchmarkBuiltin(credName, logger))
	putBuiltin(dict, "benchmark_create", benchmarkCreateBuiltin(credName, logger))
	putBuiltin(dict, "benchmark_delete", benchmarkDeleteBuiltin(credName, logger))
	putBuiltin(dict, "benchmark_baselines", benchmarkBaselinesBuiltin(credName, logger))
	putBuiltin(dict, "benchmark_catalog_rules", benchmarkCatalogRulesBuiltin(credName, logger))
	putBuiltin(dict, "benchmark_compliance_percentage", benchmarkComplianceBuiltin(credName, logger))
	putBuiltin(dict, "benchmark_report_devices", benchmarkReportDevicesBuiltin(credName, logger))
	putBuiltin(dict, "benchmark_report_rules", benchmarkReportRulesBuiltin(credName, logger))
}

func jsonResult(v interface{}, err error) (starlark.Value, error) {
	if err != nil {
		return resultErr(err.Error())
	}
	b, err := json.Marshal(v)
	if err != nil {
		return resultErr(err.Error())
	}
	val, err := JSONDecodeToStarlark(string(b))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(val), nil
}

func devicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter, sort starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("devices", args, kwargs, "filter?", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize,
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
		items, err := buddy.DeviceListValues(c, buddy.DeviceListOpts{
			Filter: string(filter), Sort: string(sort), Page: opts,
		}, logger)
		return jsonResult(items, err)
	}
}

func deviceBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("device", args, kwargs, "id", &id,
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
		out, err := buddy.DeviceGetValue(c, string(id), logger)
		return jsonResult(out, err)
	}
}

func deviceSetBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID starlark.String
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_set", args, kwargs, "device_id", &deviceID, "body", &body,
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
		if err := buddy.DeviceSetValue(c, string(deviceID), scriptOpts, []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func deviceDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_delete", args, kwargs, "device_id", &deviceID,
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
		if err := buddy.DeviceDeleteValue(c, string(deviceID), scriptOpts, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func deviceApplicationsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID, filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_applications", args, kwargs, "device_id", &deviceID, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
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
		items, err := buddy.DeviceApplicationsValue(c, string(deviceID), opts, string(filter), logger)
		return jsonResult(items, err)
	}
}

func deviceActionBuiltin(credName string, logger *slog.Logger, name string, fn func(string, string, buddy.Opts, *slog.Logger) error) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_"+name, args, kwargs, "device_id", &deviceID,
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
		if err := fn(c, string(deviceID), scriptOpts, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func deviceGroupsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_groups", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize,
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
		items, err := buddy.DeviceGroupListValues(c, buddy.DeviceGroupListOpts{
			Filter: string(filter), Page: opts,
		}, logger)
		return jsonResult(items, err)
	}
}

func deviceGroupBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_group", args, kwargs, "id", &id,
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
		out, err := buddy.DeviceGroupGetValue(c, string(id), logger)
		return jsonResult(out, err)
	}
}

func deviceGroupCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_group_create", args, kwargs, "body", &body,
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
		out, err := buddy.DeviceGroupCreateValue(c, scriptOpts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("device_group_create: cancelled")
		}
		return jsonResult(out, nil)
	}
}

func deviceGroupSetBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_group_set", args, kwargs, "group_id", &groupID, "body", &body,
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
		if err := buddy.DeviceGroupSetValue(c, string(groupID), scriptOpts, []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func deviceGroupDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_group_delete", args, kwargs, "group_id", &groupID,
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
		if err := buddy.DeviceGroupDeleteValue(c, string(groupID), scriptOpts, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func deviceGroupMembersBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_group_members", args, kwargs, "group_id", &groupID,
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
		out, err := buddy.DeviceGroupMembersValue(c, string(groupID), logger)
		return jsonResult(out, err)
	}
}

func deviceGroupSetMembersBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("device_group_set_members", args, kwargs, "group_id", &groupID, "body", &body,
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
		if err := buddy.DeviceGroupSetMembersValue(c, string(groupID), scriptOpts, []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func ddmDeclarationDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var declarationID, filter, sort starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("ddm_declaration_devices", args, kwargs, "declaration_id", &declarationID, "filter?", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize,
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
		out, err := buddy.DDMDeclarationDevicesValue(c, string(declarationID), opts, string(filter), string(sort), logger)
		return jsonResult(out, err)
	}
}

func ddmDeviceDeclarationsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID, filter, sort starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("ddm_device_declarations", args, kwargs, "device_id", &deviceID, "filter?", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize,
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
		out, err := buddy.DDMDeviceDeclarationsValue(c, string(deviceID), opts, string(filter), string(sort), logger)
		return jsonResult(out, err)
	}
}

func ddmDeviceChannelsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("ddm_device_channels", args, kwargs, "device_id", &deviceID,
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
		out, err := buddy.DDMDeviceChannelsValue(c, string(deviceID), logger)
		return jsonResult(out, err)
	}
}

func benchmarksBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmarks", args, kwargs,
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
		out, err := buddy.BenchmarkListValues(c, logger)
		return jsonResult(out, err)
	}
}

func benchmarkBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark", args, kwargs, "id", &id,
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
		out, err := buddy.BenchmarkGetValue(c, string(id), logger)
		return jsonResult(out, err)
	}
}

func benchmarkCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark_create", args, kwargs, "body", &body,
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
		out, err := buddy.BenchmarkCreateValue(c, scriptOpts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		if out == nil {
			return resultErr("benchmark_create: cancelled")
		}
		return jsonResult(out, nil)
	}
}

func benchmarkDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark_delete", args, kwargs, "benchmark_id", &benchmarkID,
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
		if err := buddy.BenchmarkDeleteValue(c, string(benchmarkID), scriptOpts, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func benchmarkBaselinesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark_baselines", args, kwargs,
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
		out, err := buddy.BenchmarkBaselinesValue(c, logger)
		return jsonResult(out, err)
	}
}

func benchmarkCatalogRulesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var baseline starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark_catalog_rules", args, kwargs, "baseline?", &baseline,
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
		out, err := buddy.BenchmarkCatalogRulesValue(c, string(baseline), logger)
		return jsonResult(out, err)
	}
}

func benchmarkComplianceBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark_compliance_percentage", args, kwargs, "benchmark_id", &benchmarkID,
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
		out, err := buddy.BenchmarkCompliancePercentageValue(c, string(benchmarkID), logger)
		return jsonResult(out, err)
	}
}

func benchmarkReportDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark_report_devices", args, kwargs, "benchmark_id", &benchmarkID,
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
		out, err := buddy.BenchmarkReportDevicesValue(c, string(benchmarkID), logger)
		return jsonResult(out, err)
	}
}

func benchmarkReportRulesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("benchmark_report_rules", args, kwargs, "benchmark_id", &benchmarkID,
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
		out, err := buddy.BenchmarkReportRulesValue(c, string(benchmarkID), logger)
		return jsonResult(out, err)
	}
}
