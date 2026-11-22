package script

import (
	"fmt"
	"log/slog"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func scriptLocalFilenameBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String
	if err := starlark.UnpackArgs("script_local_filename", args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	return resultOK(starlark.String(buddy.ScriptFilename(string(name)))), nil
}

func scriptLocalStemBuiltin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filename starlark.String
	if err := starlark.UnpackArgs("script_local_stem", args, kwargs, "filename", &filename); err != nil {
		return nil, err
	}
	stem, err := buddy.ScriptLocalStem(string(filename))
	if err != nil {
		return resultErr(err.Error())
	}
	return resultOK(starlark.String(stem)), nil
}

func scriptsCheckBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var dir starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("scripts_check", args, kwargs, "dir", &dir,
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
		rows, err := buddy.ScriptsCheck(sessionPin, string(dir), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, row := range rows {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("class"), starlark.String(row.Class))
			_ = d.SetKey(starlark.String("path"), starlark.String(row.Path))
			_ = d.SetKey(starlark.String("file"), starlark.String(row.File))
			_ = d.SetKey(starlark.String("id"), starlark.String(row.ID))
			_ = d.SetKey(starlark.String("name"), starlark.String(row.Name))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}

func scriptCheckBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var file, id starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_check", args, kwargs, "file", &file, "id?", &id,
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
		res, err := buddy.ScriptCheck(sessionPin, string(file), string(id), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("ok"), starlark.Bool(res.OK))
		_ = d.SetKey(starlark.String("class"), starlark.String(res.Class))
		_ = d.SetKey(starlark.String("id"), starlark.String(res.ID))
		_ = d.SetKey(starlark.String("name"), starlark.String(res.Name))
		_ = d.SetKey(starlark.String("file"), starlark.String(res.File))
		_ = d.SetKey(starlark.String("sha256"), starlark.String(res.SHA256))
		_ = d.SetKey(starlark.String("localSha256"), starlark.String(res.LocalSHA256))
		return resultOK(d), nil
	}
}

func scriptDiffBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var file, id, name starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_diff", args, kwargs, "file", &file, "id?", &id, "name?", &name,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		if id != "" && name != "" {
			return nil, fmt.Errorf("script_diff: pass either id= or name=, not both")
		}
		ref := string(id)
		if name != "" {
			ref = string(name)
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		res, err := buddy.ScriptDiff(sessionPin, string(file), ref, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		d := new(starlark.Dict)
		_ = d.SetKey(starlark.String("ok"), starlark.Bool(res.OK))
		_ = d.SetKey(starlark.String("class"), starlark.String(res.Class))
		_ = d.SetKey(starlark.String("id"), starlark.String(res.ID))
		_ = d.SetKey(starlark.String("name"), starlark.String(res.Name))
		_ = d.SetKey(starlark.String("file"), starlark.String(res.File))
		_ = d.SetKey(starlark.String("diff"), starlark.String(res.Diff))
		return resultOK(d), nil
	}
}

func scriptDownloadBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id, name, path starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_download", args, kwargs, "id?", &id, "path?", &path, "name?", &name,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		if (id != "") == (name != "") {
			return nil, fmt.Errorf("script_download: pass exactly one of id= or name=")
		}
		ref := string(id)
		if name != "" {
			ref = string(name)
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		written, err := buddy.ScriptDownload(sessionPin, ref, string(path), logger)
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.String(written)), nil
	}
}

func scriptRenameBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id, newName, file, from starlark.String
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_rename", args, kwargs, "id?", &id, "name", &newName, "file?", &file, "from?", &from,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		n := 0
		ref := ""
		if id != "" {
			n++
			ref = string(id)
		}
		if file != "" {
			n++
			ref = string(file)
		}
		if from != "" {
			n++
			ref = string(from)
		}
		if n != 1 {
			return nil, fmt.Errorf("script_rename: pass exactly one of id=, file=, or from=")
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		if err := buddy.ScriptRename(sessionPin, scriptOpts, ref, string(newName), logger); err != nil {
			return resultErr(err.Error())
		}
		return resultOK(starlark.None), nil
	}
}

func scriptHistoryBuiltin(credName string, logger *slog.Logger) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id, name, file starlark.String
		var limit starlark.Int
		var sess sessionArgs
		if err := starlark.UnpackArgs("script_history", args, kwargs, "id?", &id, "name?", &name, "file?", &file, "limit?", &limit,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth); err != nil {
			return nil, err
		}
		n := 0
		ref := ""
		if id != "" {
			n++
			ref = string(id)
		}
		if name != "" {
			n++
			ref = string(name)
		}
		if file != "" {
			n++
			ref = string(file)
		}
		if n != 1 {
			return nil, fmt.Errorf("script_history: pass exactly one of id=, name=, or file=")
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error())
		}
		defer restoreSession()
		resolved, err := buddy.ScriptResolve(sessionPin, ref, logger)
		if err != nil {
			return resultErr(err.Error())
		}
		lim := 0
		if i, ok := limit.Int64(); ok {
			lim = int(i)
		}
		entries, _, err := buddy.ScriptHistoryFetch(sessionPin, resolved.ID, lim)
		if err != nil {
			return resultErr(err.Error())
		}
		list := starlark.NewList(nil)
		for _, e := range entries {
			d := new(starlark.Dict)
			_ = d.SetKey(starlark.String("id"), starlark.MakeInt(e.ID))
			_ = d.SetKey(starlark.String("username"), starlark.String(e.Username))
			_ = d.SetKey(starlark.String("date"), starlark.String(e.Date))
			_ = d.SetKey(starlark.String("note"), starlark.String(e.Note))
			_ = d.SetKey(starlark.String("details"), starlark.String(e.Details))
			_ = list.Append(d)
		}
		return resultOK(list), nil
	}
}
