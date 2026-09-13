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
	Context starlark.String
	Surface starlark.String
	Auth    starlark.String
}

// sessionUnpack returns UnpackArgs trailing keys for tenant?, context?, surface?, auth?.
func sessionUnpack(s *sessionArgs) []any {
	return []any{
		"tenant?", &s.Tenant,
		"context?", &s.Context,
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
	if c := strings.TrimSpace(string(s.Context)); c != "" {
		tenant = c
	}
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

// sessionRefFromArgs builds a SessionRef from source session kwargs.
// When tenant and auth are both empty, Auth is set to the process/thread default pin
// so ResolveSessionRef matches beginSession behavior.
func sessionRefFromArgs(thread *starlark.Thread, defaultAuth string, s sessionArgs) (buddy.SessionRef, error) {
	if err := validateSurfaceArg(s.Surface); err != nil {
		return buddy.SessionRef{}, err
	}
	tenant := strings.TrimSpace(string(s.Tenant))
	if c := strings.TrimSpace(string(s.Context)); c != "" {
		tenant = c
	}
	auth := strings.TrimSpace(string(s.Auth))
	surface := strings.TrimSpace(string(s.Surface))
	if tenant == "" && auth == "" {
		auth = getAuth(thread, defaultAuth)
	}
	return buddy.SessionRef{Tenant: tenant, Auth: auth, Surface: surface}, nil
}

// destSessionRef returns the destination ref for dual-session copy.
// When all to_* are empty, destination equals source (same-tenant copy).
func destSessionRef(src buddy.SessionRef, toTenant, toSurface, toAuth starlark.String) (buddy.SessionRef, error) {
	if err := validateSurfaceArg(toSurface); err != nil {
		return buddy.SessionRef{}, err
	}
	t := strings.TrimSpace(string(toTenant))
	a := strings.TrimSpace(string(toAuth))
	s := strings.TrimSpace(string(toSurface))
	if t == "" && a == "" && s == "" {
		return src, nil
	}
	return buddy.SessionRef{Tenant: t, Auth: a, Surface: s}, nil
}

// rejectLegacyFromKwargs errors if removed from_* kwargs are passed.
func rejectLegacyFromKwargs(fromTenant, fromSurface, fromAuth starlark.String) error {
	if strings.TrimSpace(string(fromTenant)) != "" ||
		strings.TrimSpace(string(fromSurface)) != "" ||
		strings.TrimSpace(string(fromAuth)) != "" {
		return fmt.Errorf("from_tenant/from_auth/from_surface removed; use tenant=/context=/auth=/surface= for source and to_tenant=/to_context=/to_auth=/to_surface= for destination")
	}
	return nil
}

// dualSessionUnpackKeys are shared optional kwargs for copy builtins (source + dest + legacy reject).
type dualSessionKwargs struct {
	Src                                    sessionArgs
	ToTenant, ToContext, ToSurface, ToAuth starlark.String
	FromTenant, FromSurface, FromAuth      starlark.String // rejected if set
}

func (d *dualSessionKwargs) unpackTail() []any {
	return []any{
		"from_tenant?", &d.FromTenant,
		"from_surface?", &d.FromSurface,
		"from_auth?", &d.FromAuth,
		"tenant?", &d.Src.Tenant,
		"context?", &d.Src.Context,
		"surface?", &d.Src.Surface,
		"auth?", &d.Src.Auth,
		"to_tenant?", &d.ToTenant,
		"to_context?", &d.ToContext,
		"to_surface?", &d.ToSurface,
		"to_auth?", &d.ToAuth,
	}
}

func (d *dualSessionKwargs) refs(thread *starlark.Thread, defaultAuth string) (src, dst buddy.SessionRef, err error) {
	if err := rejectLegacyFromKwargs(d.FromTenant, d.FromSurface, d.FromAuth); err != nil {
		return buddy.SessionRef{}, buddy.SessionRef{}, err
	}
	src, err = sessionRefFromArgs(thread, defaultAuth, d.Src)
	if err != nil {
		return buddy.SessionRef{}, buddy.SessionRef{}, err
	}
	dst, err = destSessionRef(src, firstStarlarkString(d.ToContext, d.ToTenant), d.ToSurface, d.ToAuth)
	return src, dst, err
}

func firstStarlarkString(prefer, fallback starlark.String) starlark.String {
	if strings.TrimSpace(string(prefer)) != "" {
		return prefer
	}
	return fallback
}
