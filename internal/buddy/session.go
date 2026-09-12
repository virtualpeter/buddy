package buddy

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// OpClass classifies an API call for auth picking.
type OpClass string

const (
	OpAny           OpClass = "any"            // general / unknown
	OpRead          OpClass = "read"           // prefer readonly
	OpMutate        OpClass = "mutate"         // require full
	OpPlatform      OpClass = "platform"       // blueprints / gateway-only (read)
	OpPlatformWrite OpClass = "platform_write" // blueprints mutate — platform + full
	OpClassic       OpClass = "classic"        // Classic API GET — either surface unless classic_instance_only
	OpClassicWrite  OpClass = "classic_write"  // Classic API mutate — same toggle
	OpACS           OpClass = "acs"            // Jamf Pro ACS UI API GET — always instance (not the Classic toggle)
	OpACSWrite      OpClass = "acs_write"      // ACS mutate — always instance + full
	OpProUI         OpClass = "pro_ui"         // undocumented Pro JSON GET (policy list) — always instance
	OpProUIWrite    OpClass = "pro_ui_write"   // same paths, mutate — always instance + full
	OpSchema        OpClass = "schema"         // live Pro/Classic schema — always instance URL
	OpPro           OpClass = "pro"            // published Jamf Pro JSON API — either surface
)

// Session is a resolved tenant + auth used for a request.
type Session struct {
	TenantName string
	AuthName   string
	Tenant     TenantConfig
	Auth       AuthEntry
	Credential CredentialConfig // composed view for HTTP/auth stack
	Surface    string           // platform | instance | any actually used
}

// activeSelection holds process-wide default tenant and optional auth pin.
var activeSelection struct {
	mu      sync.RWMutex
	tenant  string
	authPin string // empty = auto-pick
	channel string // cli | mcp | ide
}

// SetActiveSelection sets the process default tenant and optional auth pin.
// Call from main after parsing --tenant / --user.
func SetActiveSelection(tenant, authPin string) {
	activeSelection.mu.Lock()
	defer activeSelection.mu.Unlock()
	activeSelection.tenant = strings.TrimSpace(tenant)
	activeSelection.authPin = strings.TrimSpace(authPin)
}

// ActiveSelection returns the process default tenant and auth pin.
func ActiveSelection() (tenant, authPin string) {
	activeSelection.mu.RLock()
	defer activeSelection.mu.RUnlock()
	return activeSelection.tenant, activeSelection.authPin
}

// SetActiveChannel sets the process buddy client channel (cli|mcp|ide) for
// tenant selection and auth channels enforcement.
func SetActiveChannel(channel string) {
	activeSelection.mu.Lock()
	defer activeSelection.mu.Unlock()
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = ChannelCLI
	}
	activeSelection.channel = channel
}

// ActiveChannel returns the process surface channel (default cli).
func ActiveChannel() string {
	activeSelection.mu.RLock()
	defer activeSelection.mu.RUnlock()
	if activeSelection.channel == "" {
		return ChannelCLI
	}
	return activeSelection.channel
}

// ResolveSession picks tenant+auth for an operation.
//
//	tenantName: override (empty → active selection / config default)
//	authName:   pin (empty → auto-pick among tenant's auths; non-empty prefers that auth)
//	op:         operation class for ranking
//
// A pin is honored when that auth can serve the op. If it cannot — wrong Jamf
// surface, or the auth is readonly and the op mutates — buddy auto-picks another
// linked auth on the same tenant. Channel access=readonly is a hard clamp (no
// fallback). Platform product writes still need a platform + full auth.
func (cfg *Config) ResolveSession(tenantName, authName string, op OpClass) (*Session, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	if op == "" {
		op = OpAny
	}

	channel := ActiveChannel()
	selTenant, selAuth := ActiveSelection()
	explicitTenant := strings.TrimSpace(tenantName) != ""
	if tenantName == "" {
		tenantName = selTenant
	}
	if tenantName == "" {
		tenantName = cfg.DefaultTenant(channel)
	}
	if authName == "" {
		authName = selAuth
	}

	// Pinned auth: tenant may be implied by the auth's tenant list.
	if authName != "" {
		if !cfg.AuthAllowed(channel, authName) {
			return nil, fmt.Errorf("auth %q is not allowed for %s (see auth channels in config)", authName, channel)
		}
		auth, err := cfg.ResolveAuth(authName)
		if err != nil {
			return nil, err
		}
		if tenantName == "" || !auth.LinkedTo(tenantName) {
			if len(auth.Tenants) == 0 {
				return nil, fmt.Errorf("auth %q has no linked tenants", authName)
			}
			if explicitTenant && tenantName != "" && !auth.LinkedTo(tenantName) {
				return nil, fmt.Errorf("auth %q is not linked to tenant %q (linked: %s)", authName, tenantName, strings.Join(auth.Tenants, ", "))
			}
			// Soft-switch: implicit tenant mismatch → first channel-allowed linked tenant.
			tenantName = ""
			for _, t := range auth.Tenants {
				if cfg.TenantAllowedOnChannel(channel, t) {
					tenantName = t
					break
				}
			}
			if tenantName == "" {
				tenantName = auth.Tenants[0]
			}
			if selTenant != "" && auth.LinkedTo(selTenant) && cfg.TenantAllowedOnChannel(channel, selTenant) {
				tenantName = selTenant
			}
		}
		if !cfg.TenantAllowedOnChannel(channel, tenantName) {
			return nil, fmt.Errorf("tenant %q is not allowed for %s (see channels.%s.tenants in config)", tenantName, channel, channel)
		}
		tenant, err := cfg.ResolveTenant(tenantName)
		if err != nil {
			return nil, err
		}
		surface := surfaceForOp(cfg, op, auth)
		surface = overlaySurfacePref(cfg, op, auth, tenant, surface, channel)
		if err := checkAuthForOp(cfg, auth, tenant, op, surface, channel); err != nil {
			if isPinFallback(err) {
				// Pin cannot serve this op — pick a compatible auth on the tenant.
				return cfg.autoPickSession(tenantName, tenant, op, channel)
			}
			return nil, fmt.Errorf("auth %q: %w", authName, err)
		}
		cred := ComposeCredential(tenant, auth, surface)
		return &Session{
			TenantName: tenantName,
			AuthName:   authName,
			Tenant:     tenant,
			Auth:       auth,
			Credential: cred,
			Surface:    surface,
		}, nil
	}

	// Auto-pick for tenant.
	if tenantName == "" {
		return nil, fmt.Errorf("no context selected — set default with 'buddy cred use <context>' or pass --context / --user")
	}
	if !cfg.TenantAllowedOnChannel(channel, tenantName) {
		return nil, fmt.Errorf("tenant %q is not allowed for %s (see channels.%s.tenants in config)", tenantName, channel, channel)
	}
	tenant, err := cfg.ResolveTenant(tenantName)
	if err != nil {
		return nil, err
	}
	return cfg.autoPickSession(tenantName, tenant, op, channel)
}

func (cfg *Config) autoPickSession(tenantName string, tenant TenantConfig, op OpClass, channel string) (*Session, error) {
	candidates := cfg.FilterAuths(channel, cfg.AuthsForTenant(tenantName))
	if len(candidates) == 0 {
		if len(cfg.AuthsForTenant(tenantName)) == 0 {
			return nil, fmt.Errorf("tenant %q has no linked auths — run 'buddy cred set auth' (see 'buddy cred help')", tenantName)
		}
		return nil, fmt.Errorf("tenant %q has no auths allowed for %s (see auth channels in config)", tenantName, channel)
	}
	bestName, bestAuth, surface, err := pickAuth(cfg, tenantName, tenant, candidates, op, channel)
	if err != nil {
		return nil, err
	}
	cred := ComposeCredential(tenant, bestAuth, surface)
	return &Session{
		TenantName: tenantName,
		AuthName:   bestName,
		Tenant:     tenant,
		Auth:       bestAuth,
		Credential: cred,
		Surface:    surface,
	}, nil
}

// requiredSurfaceForOp returns the Jamf surface this op must use, or "" when
// either instance or platform is valid (published Pro JSON / generic read-mutate).
//
// features.classic_instance_only (default false) can force Classic /JSSResource
// onto instance. When off, Classic may use /proclassic/. It does not apply
// to ACS, schema, or undocumented Pro GET /api/v1/policies — those are
// instance-only because they are not published on the Platform gateway.
func requiredSurfaceForOp(cfg *Config, op OpClass) string {
	switch op {
	case OpPlatform, OpPlatformWrite:
		return SurfacePlatform
	case OpClassic, OpClassicWrite:
		if cfg.ClassicInstanceOnly() {
			return SurfaceInstance
		}
		return ""
	case OpACS, OpACSWrite, OpSchema, OpProUI, OpProUIWrite:
		return SurfaceInstance
	default:
		return ""
	}
}

// overlaySurfacePref applies --surface= / session surface= when the op can use
// either surface. A locked Classic (classic_instance_only), ACS, undocumented
// Pro policy list, schema, or Platform-product surface is not overridden —
// otherwise a platform pin plus surface=platform would send Classic through
// /proclassic/ while the instance lock is on, ACS to the gateway (404),
// or GET /api/v1/policies through /pro/ (403).
func overlaySurfacePref(cfg *Config, op OpClass, auth AuthEntry, tenant TenantConfig, surface, channel string) string {
	pref := SurfacePreference()
	if pref != SurfacePlatform && pref != SurfaceInstance {
		return surface
	}
	if required := requiredSurfaceForOp(cfg, op); required != "" && required != pref {
		return surface
	}
	if err := checkAuthForOp(cfg, auth, tenant, op, pref, channel); err == nil {
		return pref
	}
	return surface
}

type surfaceIncompatibleError struct {
	msg string
}

func (e *surfaceIncompatibleError) Error() string { return e.msg }

func surfaceIncompatible(msg string) error {
	return &surfaceIncompatibleError{msg: msg}
}

func isSurfaceIncompatible(err error) bool {
	var e *surfaceIncompatibleError
	return errors.As(err, &e)
}

type authAccessIncompatibleError struct {
	msg string
}

func (e *authAccessIncompatibleError) Error() string { return e.msg }

func authAccessIncompatible(msg string) error {
	return &authAccessIncompatibleError{msg: msg}
}

func isAuthAccessIncompatible(err error) bool {
	var e *authAccessIncompatibleError
	return errors.As(err, &e)
}

// isPinFallback is true when a pinned auth cannot serve the op but another
// linked auth on the same tenant might (surface mismatch, or auth-level readonly).
func isPinFallback(err error) bool {
	return isSurfaceIncompatible(err) || isAuthAccessIncompatible(err)
}

func surfaceForOp(cfg *Config, op OpClass, auth AuthEntry) string {
	if s := requiredSurfaceForOp(cfg, op); s != "" {
		return s
	}
	if auth.EffectiveSurface() == SurfacePlatform {
		return SurfacePlatform
	}
	return SurfaceInstance
}

func checkAuthForOp(cfg *Config, auth AuthEntry, tenant TenantConfig, op OpClass, surface, channel string) error {
	if (op == OpMutate || op == OpPlatformWrite || op == OpClassicWrite || op == OpACSWrite || op == OpProUIWrite) && cfg.EffectiveAccessForChannel(auth, channel) == AccessReadonly {
		if cfg.ChannelAccessFor(channel) == AccessReadonly {
			return fmt.Errorf("channel %s is readonly; mutating operations are not allowed", channel)
		}
		return authAccessIncompatible("access is readonly; mutating operations require a full-access auth")
	}
	switch surface {
	case SurfacePlatform:
		if !tenant.HasPlatform() {
			return fmt.Errorf("this context has no Account environment / platform auth")
		}
		if auth.EffectiveSurface() == SurfaceInstance {
			return surfaceIncompatible("surface=instance cannot be used for platform operations")
		}
		if !auth.UsesClientCredentials() {
			return surfaceIncompatible("platform operations require type=platform")
		}
	case SurfaceInstance:
		if tenant.InstanceBaseURL() == "" && !tenant.HasInstanceURL() {
			// Allow plain URL that isn't gateway
			if strings.TrimSpace(tenant.URL) == "" || LooksLikePlatformGateway(tenant.URL) {
				return fmt.Errorf("tenant has no instance url")
			}
		}
		if auth.EffectiveSurface() == SurfacePlatform {
			return surfaceIncompatible("surface=platform cannot be used for instance/classic operations")
		}
	}
	return nil
}

func pickAuth(cfg *Config, tenantName string, tenant TenantConfig, candidates []string, op OpClass, channel string) (string, AuthEntry, string, error) {
	type scored struct {
		name    string
		auth    AuthEntry
		surface string
		score   int
	}
	var okList []scored
	var errs []string
	for _, name := range candidates {
		auth, err := cfg.ResolveAuth(name)
		if err != nil {
			errs = append(errs, name+": "+err.Error())
			continue
		}
		surface := surfaceForOp(cfg, op, auth)
		if err := checkAuthForOp(cfg, auth, tenant, op, surface, channel); err != nil {
			errs = append(errs, name+": "+err.Error())
			continue
		}
		// Also verify composed credential validates.
		cred := ComposeCredential(tenant, auth, surface)
		if err := cred.Validate(); err != nil {
			errs = append(errs, name+": "+err.Error())
			continue
		}
		pref := SurfacePreference()
		// --surface= is a bias for ops that can use either surface. Do not skip
		// auths required by Classic/schema/ACS (instance) or Platform product APIs.
		if pref != "" && requiredSurfaceForOp(cfg, op) == "" &&
			surface != pref && auth.EffectiveSurface() != pref {
			errs = append(errs, name+": surface preference "+pref+" (auth surface="+auth.EffectiveSurface()+")")
			continue
		}
		surface = overlaySurfacePref(cfg, op, auth, tenant, surface, channel)
		s := scoreAuth(cfg, name, auth, tenantName, tenant, surface, op, channel)
		if pref != "" && surface == pref {
			s += 50
		}
		okList = append(okList, scored{name: name, auth: auth, surface: surface, score: s})
	}
	if len(okList) == 0 {
		return "", AuthEntry{}, "", fmt.Errorf("no suitable auth for tenant %q op=%s: %s", tenantName, op, strings.Join(errs, "; "))
	}
	best := okList[0]
	for _, c := range okList[1:] {
		if c.score > best.score || (c.score == best.score && c.name < best.name) {
			best = c
		}
	}
	return best.name, best.auth, best.surface, nil
}

func scoreAuth(cfg *Config, authName string, auth AuthEntry, tenantName string, tenant TenantConfig, surface string, op OpClass, channel string) int {
	score := 0
	access := cfg.EffectiveAccessForChannel(auth, channel)
	surf := auth.EffectiveSurface()
	switch op {
	case OpRead:
		if access == AccessReadonly {
			score += 100
		} else {
			score += 50
		}
	case OpMutate:
		if access == AccessFull {
			score += 100
		}
	case OpPlatform, OpPlatformWrite:
		if surf == SurfacePlatform {
			score += 100
		}
		if surface == SurfacePlatform {
			score += 20
		}
		if op == OpPlatformWrite && access == AccessFull {
			score += 30
		}
	case OpClassic, OpClassicWrite, OpACS, OpACSWrite, OpSchema, OpProUI, OpProUIWrite:
		if surf == SurfaceInstance {
			score += 100
		}
		if surface == SurfaceInstance {
			score += 20
		}
		if (op == OpClassicWrite || op == OpACSWrite || op == OpProUIWrite) && access == AccessFull {
			score += 30
		}
	case OpPro, OpAny:
		if access == AccessFull {
			score += 60
		} else {
			score += 40
		}
	}
	tid := ""
	if surface == SurfacePlatform {
		tid = tenant.PlatformScopeKey()
	}
	rec := LoadCaps(authName, tenantName, surface, tid)
	if capsTokenOK(rec) {
		score += 40
		if rec != nil && rec.ProbeOk {
			score += 10
		}
	}
	if capsTokenFail(rec) {
		score -= 80
	}
	if resource := ResourceClassPreference(); resource != "" {
		if known, has := capsHasResourcePrivilege(rec, resource); known {
			if has {
				score += 25
			} else {
				score -= 15
			}
		}
	}
	return score
}

// ResolveSessionForCred is a convenience used by the HTTP stack: authName is an
// optional auth pin (Starlark auth= / MCP auth). Empty uses active selection.
func ResolveSessionForCred(authName string, op OpClass) (*Session, error) {
	if activeConfig == nil {
		return nil, fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	return activeConfig.ResolveSession("", authName, op)
}
