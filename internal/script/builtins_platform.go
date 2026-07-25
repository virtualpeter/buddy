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
		return resultErr(err.Error()), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return resultErr(err.Error()), nil
	}
	val, err := JSONDecodeToStarlark(string(b))
	if err != nil {
		return resultErr(err.Error()), nil
	}
	return resultOK(val), nil
}

func devicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter, sort, credArg starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		if err := starlark.UnpackArgs("devices", args, kwargs, "filter?", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize, "cred?", &credArg); err != nil {
			return nil, err
		}
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		items, err := buddy.DeviceListValues(resolveCred(thread, credName, credArg), buddy.DeviceListOpts{
			Filter: string(filter), Sort: string(sort), Page: opts,
		}, logger)
		return jsonResult(items, err)
	}
}

func deviceBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id, cred starlark.String
		if err := starlark.UnpackArgs("device", args, kwargs, "id", &id, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.DeviceGetValue(resolveCred(thread, credName, cred), string(id), logger)
		return jsonResult(out, err)
	}
}

func deviceSetBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID starlark.String
		var body starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("device_set", args, kwargs, "device_id", &deviceID, "body", &body, "cred?", &cred); err != nil {
			return nil, err
		}
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		if err := buddy.DeviceSetValue(resolveCred(thread, credName, cred), string(deviceID), scriptOpts, []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func deviceDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID, cred starlark.String
		if err := starlark.UnpackArgs("device_delete", args, kwargs, "device_id", &deviceID, "cred?", &cred); err != nil {
			return nil, err
		}
		if err := buddy.DeviceDeleteValue(resolveCred(thread, credName, cred), string(deviceID), scriptOpts, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func deviceApplicationsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID, filter, cred starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		if err := starlark.UnpackArgs("device_applications", args, kwargs, "device_id", &deviceID, "filter?", &filter, "page?", &page, "page_size?", &pageSize, "cred?", &cred); err != nil {
			return nil, err
		}
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		items, err := buddy.DeviceApplicationsValue(resolveCred(thread, credName, cred), string(deviceID), opts, string(filter), logger)
		return jsonResult(items, err)
	}
}

func deviceActionBuiltin(credName string, logger *slog.Logger, name string, fn func(string, string, buddy.Opts, *slog.Logger) error) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID, cred starlark.String
		if err := starlark.UnpackArgs("device_"+name, args, kwargs, "device_id", &deviceID, "cred?", &cred); err != nil {
			return nil, err
		}
		if err := fn(resolveCred(thread, credName, cred), string(deviceID), scriptOpts, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func deviceGroupsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter, credArg starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		if err := starlark.UnpackArgs("device_groups", args, kwargs, "filter?", &filter, "page?", &page, "page_size?", &pageSize, "cred?", &credArg); err != nil {
			return nil, err
		}
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		items, err := buddy.DeviceGroupListValues(resolveCred(thread, credName, credArg), buddy.DeviceGroupListOpts{
			Filter: string(filter), Page: opts,
		}, logger)
		return jsonResult(items, err)
	}
}

func deviceGroupBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id, cred starlark.String
		if err := starlark.UnpackArgs("device_group", args, kwargs, "id", &id, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.DeviceGroupGetValue(resolveCred(thread, credName, cred), string(id), logger)
		return jsonResult(out, err)
	}
}

func deviceGroupCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("device_group_create", args, kwargs, "body", &body, "cred?", &cred); err != nil {
			return nil, err
		}
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		out, err := buddy.DeviceGroupCreateValue(resolveCred(thread, credName, cred), scriptOpts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if out == nil {
			return resultErr("device_group_create: cancelled"), nil
		}
		return jsonResult(out, nil)
	}
}

func deviceGroupSetBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var body starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("device_group_set", args, kwargs, "group_id", &groupID, "body", &body, "cred?", &cred); err != nil {
			return nil, err
		}
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		if err := buddy.DeviceGroupSetValue(resolveCred(thread, credName, cred), string(groupID), scriptOpts, []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func deviceGroupDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		if err := starlark.UnpackArgs("device_group_delete", args, kwargs, "group_id", &groupID, "cred?", &cred); err != nil {
			return nil, err
		}
		if err := buddy.DeviceGroupDeleteValue(resolveCred(thread, credName, cred), string(groupID), scriptOpts, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func deviceGroupMembersBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID, cred starlark.String
		if err := starlark.UnpackArgs("device_group_members", args, kwargs, "group_id", &groupID, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.DeviceGroupMembersValue(resolveCred(thread, credName, cred), string(groupID), logger)
		return jsonResult(out, err)
	}
}

func deviceGroupSetMembersBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var groupID starlark.String
		var body starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("device_group_set_members", args, kwargs, "group_id", &groupID, "body", &body, "cred?", &cred); err != nil {
			return nil, err
		}
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		if err := buddy.DeviceGroupSetMembersValue(resolveCred(thread, credName, cred), string(groupID), scriptOpts, []byte(jsonStr), logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func ddmDeclarationDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var declarationID, filter, sort, cred starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		if err := starlark.UnpackArgs("ddm_declaration_devices", args, kwargs, "declaration_id", &declarationID, "filter?", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize, "cred?", &cred); err != nil {
			return nil, err
		}
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		out, err := buddy.DDMDeclarationDevicesValue(resolveCred(thread, credName, cred), string(declarationID), opts, string(filter), string(sort), logger)
		return jsonResult(out, err)
	}
}

func ddmDeviceDeclarationsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID, filter, sort, cred starlark.String
		page := starlark.Value(starlark.None)
		var pageSize starlark.Int
		if err := starlark.UnpackArgs("ddm_device_declarations", args, kwargs, "device_id", &deviceID, "filter?", &filter, "sort?", &sort, "page?", &page, "page_size?", &pageSize, "cred?", &cred); err != nil {
			return nil, err
		}
		opts, err := pageOptsFromArgs(page, pageSize)
		if err != nil {
			return nil, err
		}
		out, err := buddy.DDMDeviceDeclarationsValue(resolveCred(thread, credName, cred), string(deviceID), opts, string(filter), string(sort), logger)
		return jsonResult(out, err)
	}
}

func ddmDeviceChannelsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var deviceID, cred starlark.String
		if err := starlark.UnpackArgs("ddm_device_channels", args, kwargs, "device_id", &deviceID, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.DDMDeviceChannelsValue(resolveCred(thread, credName, cred), string(deviceID), logger)
		return jsonResult(out, err)
	}
}

func benchmarksBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var cred starlark.String
		if err := starlark.UnpackArgs("benchmarks", args, kwargs, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.BenchmarkListValues(resolveCred(thread, credName, cred), logger)
		return jsonResult(out, err)
	}
}

func benchmarkBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id, cred starlark.String
		if err := starlark.UnpackArgs("benchmark", args, kwargs, "id", &id, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.BenchmarkGetValue(resolveCred(thread, credName, cred), string(id), logger)
		return jsonResult(out, err)
	}
}

func benchmarkCreateBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var body starlark.Value
		var cred starlark.String
		if err := starlark.UnpackArgs("benchmark_create", args, kwargs, "body", &body, "cred?", &cred); err != nil {
			return nil, err
		}
		jsonStr, err := StarlarkToJSON(body)
		if err != nil {
			return resultErr(fmt.Sprintf("encode body as JSON: %v", err)), nil
		}
		out, err := buddy.BenchmarkCreateValue(resolveCred(thread, credName, cred), scriptOpts, []byte(jsonStr), logger)
		if err != nil {
			return resultErr(err.Error()), nil
		}
		if out == nil {
			return resultErr("benchmark_create: cancelled"), nil
		}
		return jsonResult(out, nil)
	}
}

func benchmarkDeleteBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID, cred starlark.String
		if err := starlark.UnpackArgs("benchmark_delete", args, kwargs, "benchmark_id", &benchmarkID, "cred?", &cred); err != nil {
			return nil, err
		}
		if err := buddy.BenchmarkDeleteValue(resolveCred(thread, credName, cred), string(benchmarkID), scriptOpts, logger); err != nil {
			return resultErr(err.Error()), nil
		}
		return resultOK(starlark.None), nil
	}
}

func benchmarkBaselinesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var cred starlark.String
		if err := starlark.UnpackArgs("benchmark_baselines", args, kwargs, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.BenchmarkBaselinesValue(resolveCred(thread, credName, cred), logger)
		return jsonResult(out, err)
	}
}

func benchmarkCatalogRulesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var baseline, cred starlark.String
		if err := starlark.UnpackArgs("benchmark_catalog_rules", args, kwargs, "baseline?", &baseline, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.BenchmarkCatalogRulesValue(resolveCred(thread, credName, cred), string(baseline), logger)
		return jsonResult(out, err)
	}
}

func benchmarkComplianceBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID, cred starlark.String
		if err := starlark.UnpackArgs("benchmark_compliance_percentage", args, kwargs, "benchmark_id", &benchmarkID, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.BenchmarkCompliancePercentageValue(resolveCred(thread, credName, cred), string(benchmarkID), logger)
		return jsonResult(out, err)
	}
}

func benchmarkReportDevicesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID, cred starlark.String
		if err := starlark.UnpackArgs("benchmark_report_devices", args, kwargs, "benchmark_id", &benchmarkID, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.BenchmarkReportDevicesValue(resolveCred(thread, credName, cred), string(benchmarkID), logger)
		return jsonResult(out, err)
	}
}

func benchmarkReportRulesBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var benchmarkID, cred starlark.String
		if err := starlark.UnpackArgs("benchmark_report_rules", args, kwargs, "benchmark_id", &benchmarkID, "cred?", &cred); err != nil {
			return nil, err
		}
		out, err := buddy.BenchmarkReportRulesValue(resolveCred(thread, credName, cred), string(benchmarkID), logger)
		return jsonResult(out, err)
	}
}
