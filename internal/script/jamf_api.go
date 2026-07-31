package script

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

func resultErrValue(msg string, value starlark.Value) *starlark.Dict {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("ok"), starlark.Bool(false))
	_ = d.SetKey(starlark.String("error"), starlark.String(msg))
	_ = d.SetKey(starlark.String("value"), value)
	return d
}

func jamfRequestBuiltin(credName string, logger *slog.Logger) builtinFn {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var method, path starlark.String
		var body starlark.Value = starlark.None
		var contentType, accept starlark.String
		parse := starlark.Bool(true)
		var sess sessionArgs
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"method", &method,
			"path", &path,
			"body?", &body,
			"content_type?", &contentType,
			"accept?", &accept,
			"parse?", &parse,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		return doJamfRequest(thread, c, logger, string(method), string(path), body, string(contentType), string(accept), bool(parse))
	}
}

func jamfGetBuiltin(credName string, logger *slog.Logger) builtinFn {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var path, accept starlark.String
		parse := starlark.Bool(true)
		var sess sessionArgs
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"path", &path,
			"accept?", &accept,
			"parse?", &parse,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		return doJamfRequest(thread, c, logger, "GET", string(path), starlark.None, "", string(accept), bool(parse))
	}
}

func jamfPostBuiltin(credName string, logger *slog.Logger) builtinFn {
	return jamfMutateBuiltin(credName, logger, "POST")
}

func jamfPutBuiltin(credName string, logger *slog.Logger) builtinFn {
	return jamfMutateBuiltin(credName, logger, "PUT")
}

func jamfPatchBuiltin(credName string, logger *slog.Logger) builtinFn {
	return jamfMutateBuiltin(credName, logger, "PATCH")
}

func jamfDeleteBuiltin(credName string, logger *slog.Logger) builtinFn {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var path, accept starlark.String
		parse := starlark.Bool(true)
		var sess sessionArgs
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"path", &path,
			"accept?", &accept,
			"parse?", &parse,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		return doJamfRequest(thread, c, logger, "DELETE", string(path), starlark.None, "", string(accept), bool(parse))
	}
}

func jamfMutateBuiltin(credName string, logger *slog.Logger, method string) builtinFn {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var path starlark.String
		var body starlark.Value = starlark.None
		var contentType, accept starlark.String
		parse := starlark.Bool(true)
		var sess sessionArgs
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"path", &path,
			"body?", &body,
			"content_type?", &contentType,
			"accept?", &accept,
			"parse?", &parse,
			"tenant?", &sess.Tenant,
			"surface?", &sess.Surface,
			"auth?", &sess.Auth,
		); err != nil {
			return nil, err
		}
		sessionPin, restoreSession, sessErr := beginSession(thread, credName, sess)
		if sessErr != nil {
			return resultErr(sessErr.Error()), nil
		}
		defer restoreSession()
		c := sessionPin
		return doJamfRequest(thread, c, logger, method, string(path), body, string(contentType), string(accept), bool(parse))
	}
}

func doJamfRequest(_ *starlark.Thread, credName string, logger *slog.Logger, method, path string, bodyVal starlark.Value, contentType, accept string, parse bool) (starlark.Value, error) {
	_ = logger
	if mcpGuarded() && jamfAuthTokenPath(path) {
		return resultErr(errMCPJamfTokenPath().Error()), nil
	}
	bodyBytes, inferredCT, err := encodeJamfBody(bodyVal)
	if err != nil {
		return resultErr(err.Error()), nil
	}
	if contentType == "" && inferredCT != "" {
		contentType = inferredCT
	}

	resp, err := buddy.JamfRequest(credName, buddy.JamfRequestOpts{
		Method:      method,
		Path:        path,
		Body:        bodyBytes,
		ContentType: contentType,
		Accept:      accept,
	})
	if err != nil {
		return resultErr(err.Error()), nil
	}

	value, err := jamfResponseToStarlark(resp, parse)
	if err != nil {
		return resultErr(err.Error()), nil
	}
	if resp.Status < 200 || resp.Status >= 300 {
		preview := string(resp.Body)
		if len(preview) > 500 {
			preview = preview[:500] + "…"
		}
		preview = strings.TrimSpace(preview)
		msg := fmt.Sprintf("HTTP %d %s %s", resp.Status, strings.ToUpper(method), path)
		if preview != "" {
			msg += ": " + preview
		}
		return resultErrValue(msg, value), nil
	}
	return resultOK(value), nil
}

func encodeJamfBody(bodyVal starlark.Value) ([]byte, string, error) {
	if bodyVal == nil || bodyVal == starlark.None {
		return nil, "", nil
	}
	switch v := bodyVal.(type) {
	case starlark.String:
		return []byte(string(v)), "", nil
	case starlark.Bytes:
		return []byte(v), "", nil
	case *starlark.Dict, *starlark.List:
		s, err := StarlarkToJSON(v)
		if err != nil {
			return nil, "", fmt.Errorf("encode body as JSON: %w", err)
		}
		return []byte(s), "application/json", nil
	default:
		return nil, "", fmt.Errorf("body must be str, bytes, dict, list, or None; got %s", bodyVal.Type())
	}
}

func jamfResponseToStarlark(resp *buddy.JamfResponse, parse bool) (starlark.Value, error) {
	d := new(starlark.Dict)
	_ = d.SetKey(starlark.String("status"), starlark.MakeInt(resp.Status))
	_ = d.SetKey(starlark.String("content_type"), starlark.String(resp.ContentType))

	raw := string(resp.Body)
	if !utf8.ValidString(raw) {
		_ = d.SetKey(starlark.String("body"), starlark.Bytes(resp.Body))
		return d, nil
	}

	bodyVal := starlark.Value(starlark.String(raw))
	if parse && shouldParseJSONBody(resp.ContentType, resp.Body) {
		parsed, err := JSONDecodeToStarlark(raw)
		if err == nil {
			bodyVal = parsed
		}
	}
	_ = d.SetKey(starlark.String("body"), bodyVal)
	return d, nil
}

func shouldParseJSONBody(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "json") {
		return true
	}
	if ct != "" && !strings.Contains(ct, "text/plain") {
		return false
	}
	s := strings.TrimLeftFunc(string(body), unicode.IsSpace)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}
