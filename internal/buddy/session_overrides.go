package buddy

import (
	"fmt"
	"strings"
	"sync"
)

// SessionOverrides are per-call tenant / auth / surface overrides (MCP tools, Starlark kwargs).
type SessionOverrides struct {
	Tenant  string // empty = keep current / default
	Auth    string // empty = keep current pin, or auto-pick when Tenant changes without Auth
	Surface string // instance | platform | "" (no bias)
}

var activeSurfacePref struct {
	mu      sync.RWMutex
	surface string
}

// SurfacePreference returns the call-scoped surface bias for auth auto-pick.
func SurfacePreference() string {
	activeSurfacePref.mu.RLock()
	defer activeSurfacePref.mu.RUnlock()
	return activeSurfacePref.surface
}

func setSurfacePreference(surface string) {
	activeSurfacePref.mu.Lock()
	defer activeSurfacePref.mu.Unlock()
	activeSurfacePref.surface = strings.ToLower(strings.TrimSpace(surface))
}

// BeginSessionOverrides applies tenant/auth/surface until restore() is called.
// Prefer WithSessionOverrides when a single fn closure is natural.
func BeginSessionOverrides(o SessionOverrides) (restore func(), err error) {
	prevT, prevA := ActiveSelection()
	prevS := SurfacePreference()

	tenant := strings.TrimSpace(o.Tenant)
	auth := strings.TrimSpace(o.Auth)
	surface := strings.ToLower(strings.TrimSpace(o.Surface))
	if surface != "" && surface != SurfaceInstance && surface != SurfacePlatform {
		return nil, fmt.Errorf("surface must be %q or %q (got %q)", SurfaceInstance, SurfacePlatform, surface)
	}

	newT, newA := prevT, prevA
	if tenant != "" {
		newT = tenant
		if auth == "" {
			// Switching tenant without an auth pin → auto-pick for that tenant.
			newA = ""
		}
	}
	if auth != "" {
		newA = auth
	}

	SetActiveSelection(newT, newA)
	setSurfacePreference(surface)
	var once sync.Once
	return func() {
		once.Do(func() {
			SetActiveSelection(prevT, prevA)
			setSurfacePreference(prevS)
		})
	}, nil
}

// WithSessionOverrides applies tenant/auth/surface for the duration of fn, then restores.
// Safe for single-threaded MCP/stdio; nested calls restore correctly via defer.
func WithSessionOverrides(o SessionOverrides, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("WithSessionOverrides: nil func")
	}
	restore, err := BeginSessionOverrides(o)
	if err != nil {
		return err
	}
	defer restore()
	return fn()
}

// EffectiveAuthPin returns the auth pin to pass into buddy APIs for this call.
// Prefer explicit Auth override; else active selection pin; else defaultAuth (session default).
func EffectiveAuthPin(defaultAuth string, authOverride string) string {
	if s := strings.TrimSpace(authOverride); s != "" {
		return s
	}
	_, pin := ActiveSelection()
	if pin != "" {
		return pin
	}
	return strings.TrimSpace(defaultAuth)
}
