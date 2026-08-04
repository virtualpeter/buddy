package script

import (
	"fmt"
	"strings"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
)

const (
	localAuth   = "auth"
	localTenant = "tenant"
)

// sessionArgs holds optional trailing session kwargs for Jamf builtins.
type sessionArgs struct {
	Tenant  starlark.String
	Surface starlark.String
	Auth    starlark.String
}

// sessionUnpack returns UnpackArgs trailing keys for tenant?, surface?, auth?.
func sessionUnpack(s *sessionArgs) []any {
	return []any{
		"tenant?", &s.Tenant,
		"surface?", &s.Surface,
		"auth?", &s.Auth,
	}
}

func getAuth(thread *starlark.Thread, defaultAuth string) string {
	if thread != nil && thread.Local(localAuth) != nil {
		if s, ok := thread.Local(localAuth).(string); ok {
			return s
		}
	}
	return defaultAuth
}

func validateSurfaceArg(surface starlark.String) error {
	s := strings.ToLower(strings.TrimSpace(string(surface)))
	if s == "" || s == buddy.SurfaceInstance || s == buddy.SurfacePlatform {
		return nil
	}
	return fmt.Errorf("surface must be %q or %q", buddy.SurfaceInstance, buddy.SurfacePlatform)
}

// beginSession applies session kwargs and returns the auth pin for buddy APIs plus restore.
// Always call defer restore() after a nil error.
func beginSession(thread *starlark.Thread, defaultAuth string, s sessionArgs) (pin string, restore func(), err error) {
	if err := validateSurfaceArg(s.Surface); err != nil {
		return "", nil, err
	}
	tenant := strings.TrimSpace(string(s.Tenant))
	surface := strings.TrimSpace(string(s.Surface))
	auth := strings.TrimSpace(string(s.Auth))

	o := buddy.SessionOverrides{
		Tenant:  tenant,
		Auth:    auth,
		Surface: surface,
	}
	if tenant == "" && auth == "" {
		o.Auth = getAuth(thread, defaultAuth)
	}

	pin = auth
	if pin == "" && tenant == "" {
		pin = getAuth(thread, defaultAuth)
	}

	restore, err = buddy.BeginSessionOverrides(o)
	if err != nil {
		return "", nil, err
	}
	return pin, restore, nil
}

// withSession runs fn under session overrides (MCP / multi-step helpers).
func withSession(thread *starlark.Thread, defaultAuth string, s sessionArgs, fn func(authPin string) error) error {
	pin, restore, err := beginSession(thread, defaultAuth, s)
	if err != nil {
		return err
	}
	defer restore()
	return fn(pin)
}

// resultWithSession wraps withSession; session override errors raise like other builtins.
func resultWithSession(thread *starlark.Thread, defaultAuth string, s sessionArgs, fn func(authPin string) (starlark.Value, error)) (starlark.Value, error) {
	var out starlark.Value
	err := withSession(thread, defaultAuth, s, func(authPin string) error {
		v, e := fn(authPin)
		out = v
		return e
	})
	if err != nil {
		return resultErr(err.Error())
	}
	return out, nil
}
