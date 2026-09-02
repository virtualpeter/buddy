package buddy

import (
	"fmt"
	"strings"
)

// SessionRef identifies a Jamf session for dual-session APIs (copy/migrate).
// Empty Tenant/Auth mean “use process ActiveSelection / auto-pick” until resolved.
type SessionRef struct {
	Tenant  string
	Auth    string
	Surface string
}

// Equal reports whether two refs name the same tenant+auth (after trim).
func (r SessionRef) Equal(o SessionRef) bool {
	return strings.TrimSpace(r.Tenant) == strings.TrimSpace(o.Tenant) &&
		strings.TrimSpace(r.Auth) == strings.TrimSpace(o.Auth)
}

// ResolveSessionRef applies overrides for ref, resolves tenant+auth for op, restores
// ActiveSelection, and returns a concrete ref (both Tenant and Auth set).
func ResolveSessionRef(ref SessionRef, op OpClass) (SessionRef, error) {
	restore, err := BeginSessionOverrides(SessionOverrides{
		Tenant:  ref.Tenant,
		Auth:    ref.Auth,
		Surface: ref.Surface,
	})
	if err != nil {
		return SessionRef{}, err
	}
	defer restore()

	sess, err := ResolveSessionForCred(strings.TrimSpace(ref.Auth), op)
	if err != nil {
		return SessionRef{}, err
	}
	if sess == nil {
		return SessionRef{}, fmt.Errorf("internal error: nil session")
	}
	return SessionRef{
		Tenant:  sess.TenantName,
		Auth:    sess.AuthName,
		Surface: sess.Surface,
	}, nil
}

// WithSessionRef sets ActiveSelection to ref for the duration of fn and passes the
// auth pin (ref.Auth) to fn. Prefer a resolved ref from ResolveSessionRef so tenant
// auto-pick does not depend on ambient selection.
func WithSessionRef(ref SessionRef, fn func(authPin string) error) error {
	if fn == nil {
		return fmt.Errorf("WithSessionRef: nil func")
	}
	return WithSessionOverrides(SessionOverrides{
		Tenant:  ref.Tenant,
		Auth:    ref.Auth,
		Surface: ref.Surface,
	}, func() error {
		return fn(strings.TrimSpace(ref.Auth))
	})
}
