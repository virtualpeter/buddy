package script

import (
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerAppBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "apps", appsBuiltin(credName, logger))
	putBuiltin(dict, "app", appBuiltin(credName, logger))
	putBuiltin(dict, "app_upgrade", appUpgradeBuiltin(credName, logger))
}

func appsBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var filter starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("apps", args, kwargs,
			"filter?", &filter,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		apps, err := buddy.AppSearch(pin, string(filter), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := make([]starlark.Value, 0, len(apps))
		for _, a := range apps {
			list = append(list, policyAppSummaryDict(a))
		}
		return resultOK(starlark.NewList(list)), nil
	}
}

func appBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("app", args, kwargs,
			"name", &name,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		d, err := buddy.AppGet(pin, strings.TrimSpace(string(name)), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return policyAppDetailResult(d)
	}
}

func appUpgradeBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name, version, packagePattern starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("app_upgrade", args, kwargs,
			"name", &name, "version", &version,
			"package_pattern?", &packagePattern,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		opts := scriptOpts
		if err := buddy.AppUpgrade(pin, opts, string(name), string(version), string(packagePattern), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func policyAppSummaryDict(a buddy.PolicyAppSummary) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("name"), starlark.String(a.Name))
	_ = d.SetKey(starlark.String("kind"), starlark.String(string(a.Kind)))
	_ = d.SetKey(starlark.String("version"), starlark.String(a.Version))
	_ = d.SetKey(starlark.String("ss_policy_id"), starlark.MakeInt(a.SSPolicyID))
	_ = d.SetKey(starlark.String("ss_policy_name"), starlark.String(a.SSPolicyName))
	_ = d.SetKey(starlark.String("install_trigger"), starlark.String(a.InstallTrigger))
	_ = d.SetKey(starlark.String("install_policy_id"), starlark.MakeInt(a.InstallPolicyID))
	_ = d.SetKey(starlark.String("cache_policy_id"), starlark.MakeInt(a.CachePolicyID))
	_ = d.SetKey(starlark.String("package_id"), starlark.MakeInt(a.PackageID))
	_ = d.SetKey(starlark.String("package_name"), starlark.String(a.PackageName))
	return d
}

func policyAppDetailResult(detail *buddy.PolicyAppDetail) (starlark.Value, error) {
	return jsonResult(detail, nil)
}
