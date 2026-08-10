package script

import (
	"fmt"
	"log/slog"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerAppBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "apps", appsBuiltin(credName, logger))
	putBuiltin(dict, "app", appBuiltin(credName, logger))
	putBuiltin(dict, "app_upgrade", appUpgradeBuiltin(credName, logger))
	putBuiltin(dict, "app_reconcile", appReconcileBuiltin(credName, logger))
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
		_ = pin
		apps, err := buddy.AppListImported(buddy.DefaultAppsDir(), string(filter))
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

func appReconcileBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var desiredDict *starlark.Dict
		var apply starlark.Bool
		var sess sessionArgs
		if err := starlark.UnpackArgs("app_reconcile", args, kwargs,
			"desired", &desiredDict,
			"apply?", &apply,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		desired, err := appDesiredFromStarlark(desiredDict)
		if err != nil {
			return resultErr(err.Error())
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()

		live, err := buddy.AppGet(pin, desired.Name, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		plan, err := buddy.BuildAppReconcilePlan(live, desired)
		if err != nil {
			return resultErr(err.Error())
		}

		change := changeBatchFromReconcile(plan)
		steps := make([]starlark.Value, 0, len(plan.Steps))
		for _, st := range plan.Steps {
			d, err := reconcileStepToStarlark(st)
			if err != nil {
				return resultErr(err.Error())
			}
			steps = append(steps, d)
		}

		md, fps, err := planChange(pin, change, steps, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		fmt.Print(md)

		if !bool(apply) {
			if plan.Noop {
				fmt.Println("No changes needed.")
			} else {
				fmt.Println("Plan only. Re-run with apply=True (or: buddy this-script.buddy -- apply)")
			}
			return resultOK(plannedDict(md, fps)), nil
		}

		if plan.Noop {
			fmt.Println("No changes needed.")
			return resultOK(plannedDict(md, fps)), nil
		}

		outMD, err := runChange(pin, change, steps, fps, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		_ = outMD
		return resultOK(starlark.None), nil
	}
}

func appDesiredFromStarlark(d *starlark.Dict) (buddy.AppDesired, error) {
	if d == nil {
		return buddy.AppDesired{}, fmt.Errorf("desired must be a dict")
	}
	get := func(key string) string {
		v, found, err := d.Get(starlark.String(key))
		if err != nil || !found {
			return ""
		}
		s, ok := v.(starlark.String)
		if !ok {
			return ""
		}
		return string(s)
	}
	des := buddy.AppDesired{
		Name:           get("name"),
		Kind:           get("kind"),
		Version:        get("version"),
		InstallTrigger: get("install_trigger"),
		CacheTrigger:   get("cache_trigger"),
		UninstallTrigger: get("uninstall_trigger"),
		PackagePattern: get("package_pattern"),
		Title:          get("title"),
		Summary:        get("summary"),
		Risk:           get("risk"),
		Window:         get("window"),
	}
	if strings.TrimSpace(des.Name) == "" {
		return des, fmt.Errorf("desired.name is required")
	}
	if strings.TrimSpace(des.Version) == "" {
		return des, fmt.Errorf("desired.version is required")
	}
	return des, nil
}

func changeBatchFromReconcile(plan *buddy.AppReconcilePlan) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("type"), starlark.String("change_batch"))
	_ = d.SetKey(starlark.String("title"), starlark.String(plan.Title))
	if plan.Summary != "" {
		_ = d.SetKey(starlark.String("summary"), starlark.String(plan.Summary))
	}
	if plan.Risk != "" {
		_ = d.SetKey(starlark.String("risk"), starlark.String(plan.Risk))
	}
	if plan.Window != "" {
		_ = d.SetKey(starlark.String("window"), starlark.String(plan.Window))
	}
	return d
}

func reconcileStepToStarlark(st buddy.AppReconcileStep) (*starlark.Dict, error) {
	switch st.Type {
	case "note":
		return stepDict(stepTypeNote, "note", st.Note), nil
	case "set_package":
		d := stepDict(stepTypeSetPackage,
			"name", st.Name,
			"version", st.Version,
			"from_trigger", st.FromTrigger,
		)
		if st.PackagePattern != "" {
			_ = d.SetKey(starlark.String("package_pattern"), starlark.String(st.PackagePattern))
		}
		if st.KeepSource {
			_ = d.SetKey(starlark.String("keep_source"), starlark.True)
		}
		if st.Note != "" {
			_ = d.SetKey(starlark.String("note"), starlark.String(st.Note))
		}
		return d, nil
	case "upgrade_app":
		d := stepDict(stepTypeUpgradeApp,
			"name", st.Name,
			"version", st.Version,
		)
		if st.PackagePattern != "" {
			_ = d.SetKey(starlark.String("package_pattern"), starlark.String(st.PackagePattern))
		}
		if st.Note != "" {
			_ = d.SetKey(starlark.String("note"), starlark.String(st.Note))
		}
		return d, nil
	case "set_ss":
		d := stepDict(stepTypeSetSS,
			"policy_id", st.PolicyID,
			"version", st.Version,
		)
		if st.Note != "" {
			_ = d.SetKey(starlark.String("note"), starlark.String(st.Note))
		}
		return d, nil
	default:
		return nil, fmt.Errorf("unknown reconcile step type %q", st.Type)
	}
}

func plannedDict(md string, fps []string) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("markdown"), starlark.String(md))
	list := make([]starlark.Value, len(fps))
	for i, fp := range fps {
		list[i] = starlark.String(fp)
	}
	_ = d.SetKey(starlark.String("fingerprints"), starlark.NewList(list))
	return d
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
