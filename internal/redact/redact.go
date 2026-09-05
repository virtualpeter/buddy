// Package redact filters secrets out of logs and HTTP/script traces.
// stdout of `buddy auth token show --raw` is not filtered.
package redact

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// Placeholder is substituted for recognised secret values.
const Placeholder = "<redacted>"

// Secret JSON / form / XML field names (token_type is intentionally omitted).
const secretFieldAlt = `(?:access_token|accessToken|refresh_token|refreshToken|id_token|idToken|client_secret|clientSecret|password|unlock_token|unlockToken|token)`

var (
	jsonSecretRe = regexp.MustCompile(`(?i)("` + secretFieldAlt + `")(\s*:\s*")(?:[^"\\]|\\.)*(")`)
	formSecretRe = regexp.MustCompile(`(?i)(^|[?&])(` + secretFieldAlt + `=)([^&\s]*)`)
	xmlSecretRe  = regexp.MustCompile(`(?i)<(` + secretFieldAlt + `)>[^<]*</` + secretFieldAlt + `>`)
	bearerRe     = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-+/=]{16,}`)
	jwtRe        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)
)

var secretAttrCanon = map[string]struct{}{
	"authorization":      {},
	"proxyauthorization": {},
	"cookie":             {},
	"setcookie":          {},
	"token":              {},
	"accesstoken":        {},
	"refreshtoken":       {},
	"idtoken":            {},
	"clientsecret":       {},
	"password":           {},
	"unlocktoken":        {},
}

func canonAttr(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "-", "")
	return name
}

func secretAttrName(name string) bool {
	_, ok := secretAttrCanon[canonAttr(name)]
	return ok
}

// String redacts JWTs, Bearer values, and known secret fields in s.
func String(s string) string {
	if s == "" {
		return s
	}
	s = jsonSecretRe.ReplaceAllString(s, `${1}${2}`+Placeholder+`${3}`)
	s = formSecretRe.ReplaceAllString(s, `${1}${2}`+Placeholder)
	s = xmlSecretRe.ReplaceAllString(s, `<$1>`+Placeholder+`</$1>`)
	s = bearerRe.ReplaceAllString(s, "Bearer "+Placeholder)
	s = jwtRe.ReplaceAllString(s, Placeholder)
	return s
}

// Bytes is String over a byte slice.
func Bytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte(String(string(b)))
}

// Logger returns l with a redacting handler. Nil stays nil.
func Logger(l *slog.Logger) *slog.Logger {
	if l == nil {
		return nil
	}
	return slog.New(WrapHandler(l.Handler()))
}

type handler struct {
	inner slog.Handler
}

// WrapHandler redacts messages and attribute values. Wrapping twice is a no-op.
func WrapHandler(inner slog.Handler) slog.Handler {
	if inner == nil {
		return nil
	}
	if _, ok := inner.(*handler); ok {
		return inner
	}
	return &handler{inner: inner}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, String(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, nr)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return &handler{inner: h.inner.WithAttrs(out)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{inner: h.inner.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	switch a.Value.Kind() {
	case slog.KindString:
		a.Value = slog.StringValue(redactNamed(a.Key, a.Value.String()))
	case slog.KindGroup:
		g := a.Value.Group()
		out := make([]slog.Attr, len(g))
		for i, ga := range g {
			out[i] = redactAttr(ga)
		}
		a.Value = slog.GroupValue(out...)
	case slog.KindAny:
		a.Value = redactAny(a.Key, a.Value.Any())
	default:
		if secretAttrName(a.Key) {
			a.Value = slog.StringValue(Placeholder)
		}
	}
	return a
}

func redactNamed(key, value string) string {
	if secretAttrName(key) {
		return Placeholder
	}
	return String(value)
}

func redactAny(key string, v any) slog.Value {
	switch t := v.(type) {
	case string:
		return slog.StringValue(redactNamed(key, t))
	case []byte:
		return slog.StringValue(String(string(t)))
	case error:
		return slog.StringValue(String(t.Error()))
	default:
		s := fmt.Sprint(v)
		if red := String(s); red != s {
			return slog.StringValue(red)
		}
		return slog.AnyValue(v)
	}
}
