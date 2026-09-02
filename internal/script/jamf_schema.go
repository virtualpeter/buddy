package script

import (
	"log/slog"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

// jamf_schema(api?="all", filter?="", full?=False, tenant?, surface?, auth?)
//
// Fetches Jamf Pro OpenAPI (/api/schema) and/or Classic Swagger
// (/classicapi/doc/swagger.yaml) live from the credential's server, plus the
// embedded Platform API Gateway specs. api: "pro", "classic", "platform",
// "both" (pro+classic), or "all" (default; pro+classic+platform). Platform
// specs are bundled in the binary (no live gateway schema endpoint), so
// api="platform" works without a credential. The Pro/Classic schema documents
// are served publicly by the Jamf host and fetched with no Authorization header;
// the credential is only used to resolve the base URL, so schema lookup works
// even when the credential has no stored secret.
// filter is a case-insensitive AND of space-separated terms matched against path,
// method, tags, summary, operationId, description.
// Default value is a compact operations catalog; full=True includes schema document(s).
func jamfSchemaBuiltin(credName string, logger *slog.Logger) builtinFn {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		api := starlark.String("all")
		var filter starlark.String
		full := starlark.Bool(false)
		var sess sessionArgs
		if err := starlark.UnpackArgs(b.Name(), args, kwargs,
			"api?", &api,
			"filter?", &filter,
			"full?", &full,
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
		c := sessionPin
		_ = logger
		res, err := buddy.FetchJamfSchema(c, string(api), string(filter), bool(full))
		if err != nil {
			return resultErr(err.Error())
		}
		val, err := GoValueToStarlark(schemaResultToMap(res))
		if err != nil {
			return resultErr(err.Error())
		}
		return resultOK(val), nil
	}
}

func schemaResultToMap(res *buddy.SchemaResult) map[string]interface{} {
	out := map[string]interface{}{
		"api":        res.API,
		"count":      res.Count,
		"operations": schemaOpsToSlice(res.Operations),
	}
	if res.Source != "" {
		out["source"] = res.Source
	}
	if res.Title != "" {
		out["title"] = res.Title
	}
	if res.Version != "" {
		out["version"] = res.Version
	}
	if res.Spec != "" {
		out["spec"] = res.Spec
	}
	if res.Document != nil {
		out["document"] = res.Document
	}
	if res.Pro != nil {
		out["pro"] = schemaResultToMap(res.Pro)
	}
	if res.Classic != nil {
		out["classic"] = schemaResultToMap(res.Classic)
	}
	if res.Platform != nil {
		out["platform"] = schemaResultToMap(res.Platform)
	}
	return out
}

func schemaOpsToSlice(ops []buddy.SchemaOperation) []interface{} {
	out := make([]interface{}, 0, len(ops))
	for _, op := range ops {
		m := map[string]interface{}{
			"path":   op.Path,
			"method": op.Method,
		}
		if op.API != "" {
			m["api"] = op.API
		}
		if op.Summary != "" {
			m["summary"] = op.Summary
		}
		if op.Description != "" {
			m["description"] = op.Description
		}
		if op.OperationID != "" {
			m["operationId"] = op.OperationID
		}
		if len(op.Tags) > 0 {
			tags := make([]interface{}, len(op.Tags))
			for i, t := range op.Tags {
				tags[i] = t
			}
			m["tags"] = tags
		}
		out = append(out, m)
	}
	return out
}
