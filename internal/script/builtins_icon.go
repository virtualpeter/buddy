package script

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func registerIconBuiltins(dict starlark.StringDict, credName string, logger *slog.Logger) {
	putBuiltin(dict, "icon", iconBuiltin(credName, logger))
	putBuiltin(dict, "icon_upload", iconUploadBuiltin(credName, logger))
	putBuiltin(dict, "icon_check", iconCheckBuiltin(credName, logger))
	putBuiltin(dict, "icon_download", iconDownloadBuiltin(credName, logger))
}

func iconStarlarkDict(icon buddy.Icon) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("id"), starlark.MakeInt(icon.ID))
	_ = d.SetKey(starlark.String("name"), starlark.String(icon.Name))
	_ = d.SetKey(starlark.String("url"), starlark.String(icon.URL))
	return d
}

func iconBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("icon", args, kwargs, "id", &idVal,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		id, err := starlarkIDInt(idVal, "id")
		if err != nil {
			return resultErr(err.Error())
		}
		icon, err := buddy.IconGet(pin, id)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(iconStarlarkDict(icon)), nil
	}
}

func iconUploadBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var file starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("icon_upload", args, kwargs, "file", &file,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		icon, err := buddy.IconUpload(pin, string(file))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(iconStarlarkDict(icon)), nil
	}
}

func iconCheckBuiltin(credName string, _ *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var file starlark.String
		var iconIDVal starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("icon_check", args, kwargs, "file", &file,
			"icon_id?", &iconIDVal, "tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		hasIconID := iconIDVal != nil && iconIDVal != starlark.None
		hasSess := strings.TrimSpace(string(sess.Tenant)) != "" || strings.TrimSpace(string(sess.Surface)) != "" || strings.TrimSpace(string(sess.Auth)) != ""
		if hasSess && !hasIconID {
			return resultErr("icon_id= is required when tenant=/surface=/auth= are set")
		}
		iconID := 0
		if hasIconID {
			id, err := starlarkIDInt(iconIDVal, "icon_id")
			if err != nil {
				return resultErr(err.Error())
			}
			iconID = id
		}
		var res buddy.IconCheckResult
		var err error
		if iconID <= 0 {
			res, err = buddy.IconCheck(string(file))
		} else {
			pin, restore, sessErr := beginSession(thread, credName, sess)
			if sessErr != nil {
				return resultErr(sessErr.Error())
			}
			defer restore()
			res, err = buddy.IconCheckFile(pin, string(file), iconID)
		}
		if err != nil {
			return resultErr(err.Error())
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("url"), starlark.String(res.URL))
		_ = d.SetKey(starlark.String("present"), starlark.Bool(res.Present))
		_ = d.SetKey(starlark.String("sha256"), starlark.String(res.SHA256))
		_ = d.SetKey(starlark.String("ics"), starlark.Bool(res.ICS))
		if res.IconID > 0 {
			_ = d.SetKey(starlark.String("icon_id"), starlark.MakeInt(res.IconID))
		}
		return resultOK(d), nil
	}
}

func iconDownloadBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var path starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("icon_download", args, kwargs, "id", &idVal, "path", &path,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		id, err := starlarkIDInt(idVal, "id")
		if err != nil {
			return resultErr(err.Error())
		}
		data, err := buddy.IconDownload(pin, id)
		if err != nil {
			return resultErr(err.Error())
		}
		if err := os.WriteFile(string(path), data, 0o644); err != nil {
			return resultErr(fmt.Sprintf("write icon file: %s", err))
		}
		return resultOK(starlark.None), nil
	}
}

func policySetIconBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var idVal starlark.Value = starlark.None
		var file starlark.String
		var iconIDVal starlark.Value = starlark.None
		var sess sessionArgs
		if err := starlark.UnpackArgs("policy_set_icon", args, kwargs,
			"id", &idVal, "file?", &file, "icon_id?", &iconIDVal,
			"tenant?", &sess.Tenant, "surface?", &sess.Surface, "auth?", &sess.Auth); err != nil {
			return nil, err
		}
		filePath := strings.TrimSpace(string(file))
		hasFile := filePath != ""
		hasIconID := iconIDVal != nil && iconIDVal != starlark.None
		if hasFile == hasIconID {
			return resultErr("pass either file= or icon_id=, not both")
		}
		pin, restore, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restore()
		pid, err := starlarkIDInt(idVal, "id")
		if err != nil {
			return resultErr(err.Error())
		}
		if hasFile {
			if err := buddy.PolicySetIcon(pin, scriptOpts, pid, filePath, logger); err != nil {
				return resultErr(err.Error())
			}
			return resultOK(starlark.None), nil
		}
		iconID, err := starlarkIDInt(iconIDVal, "icon_id")
		if err != nil {
			return resultErr(err.Error())
		}
		if err := buddy.PolicySetIconID(pin, scriptOpts, pid, iconID, logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}
