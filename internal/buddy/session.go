package buddy

import (
	"fmt"
	"strings"
	"sync"
)

// OpClass classifies an API call for auth picking.
type OpClass string

const (
	OpAny          OpClass = "any"           // general / unknown
	OpRead         OpClass = "read"          // prefer readonly
	OpMutate       OpClass = "mutate"        // require full
	OpPlatform     OpClass = "platform"      // blueprints / gateway-only (read)
	OpPlatformWrite OpClass = "platform_write" // blueprints mutate — platform + full
	OpClassic      OpClass = "classic"       // Classic API — prefer instance
	OpSchema       OpClass = "schema"        // live Pro/Classic schema — need instance URL
	OpPro          OpClass = "pro"           // Jamf Pro JSON API — either surface
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
	mu         sync.RWMutex
	tenant     string
	authPin    string // empty = auto-pick
	channel    string // cli | mcp | serve
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

// SetActiveChannel sets the process buddy client channel (cli|mcp|serve) for
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
//	authName:   pin (empty → auto-pick among tenant's auths; non-empty pins that auth)
//	op:         operation class for ranking
func (cfg *Config) ResolveSession(tenantName, authName string, op OpClass) (*Session, error) {
	if cfg == nil {
		return nil, fmt.Errorf("internal error: config is nil")
	}
	if op == "" {
		op = OpAny
	}

	channel := ActiveChannel()
	selTenant, selAuth := ActiveSelection()
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
			if tenantName != "" && !auth.LinkedTo(tenantName) {
				return nil, fmt.Errorf("auth %q is not linked to tenant %q (linked: %s)", authName, tenantName, strings.Join(auth.Tenants, ", "))
			}
			tenantName = auth.Tenants[0]
			if selTenant != "" && auth.LinkedTo(selTenant) {
				tenantName = selTenant
			}
		}
		tenant, err := cfg.ResolveTenant(tenantName)
		if err != nil {
			return nil, err
		}
		surface := surfaceForOp(op, auth, tenant)
		if err := checkAuthForOp(auth, tenant, op, surface); err != nil {
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
		return nil, fmt.Errorf("no tenant selected — set default with 'buddy cred use <tenant>' or pass --tenant / --user")
	}
	tenant, err := cfg.ResolveTenant(tenantName)
	if err != nil {
		return nil, err
	}
	candidates := cfg.FilterAuths(channel, cfg.AuthsForTenant(tenantName))
	if len(candidates) == 0 {
		if len(cfg.AuthsForTenant(tenantName)) == 0 {
			return nil, fmt.Errorf("tenant %q has no linked auths — run 'buddy cred set auth' (see 'buddy cred help')", tenantName)
		}
		return nil, fmt.Errorf("tenant %q has no auths allowed for %s (see auth channels in config)", tenantName, channel)
	}
	bestName, bestAuth, surface, err := pickAuth(cfg, tenantName, tenant, candidates, op)
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

func surfaceForOp(op OpClass, auth AuthEntry, tenant TenantConfig) string {
	switch op {
	case OpPlatform, OpPlatformWrite:
		return SurfacePlatform
	case OpClassic, OpSchema:
		return SurfaceInstance
	case OpPro, OpRead, OpMutate, OpAny:
		switch auth.EffectiveSurface() {
		case SurfacePlatform:
			return SurfacePlatform
		case SurfaceInstance:
			return SurfaceInstance
		default:
			if tenant.HasPlatform() && (op == OpPlatform || (!tenant.HasInstanceURL() && tenant.HasPlatform())) {
				return SurfacePlatform
			}
			if tenant.HasInstanceURL() {
				return SurfaceInstance
			}
			if tenant.HasPlatform() {
				return SurfacePlatform
			}
			return SurfaceAny
		}
	default:
		return auth.EffectiveSurface()
	}
}

func checkAuthForOp(auth AuthEntry, tenant TenantConfig, op OpClass, surface string) error {
	if (op == OpMutate || op == OpPlatformWrite) && auth.EffectiveAccess() == AccessReadonly {
		return fmt.Errorf("access is readonly; mutating operations require a full-access auth")
	}
	switch surface {
	case SurfacePlatform:
		if !tenant.HasPlatform() {
			return fmt.Errorf("tenant has no platform tenantId/region")
		}
		if auth.EffectiveSurface() == SurfaceInstance {
			return fmt.Errorf("surface=instance cannot be used for platform operations")
		}
		if auth.Type != "oauth" {
			return fmt.Errorf("platform operations require oauth auth")
		}
	case SurfaceInstance:
		if tenant.InstanceBaseURL() == "" && !tenant.HasInstanceURL() {
			// Allow plain URL that isn't gateway
			if strings.TrimSpace(tenant.URL) == "" || LooksLikePlatformGateway(tenant.URL) {
				return fmt.Errorf("tenant has no instance url")
			}
		}
		if auth.EffectiveSurface() == SurfacePlatform {
			return fmt.Errorf("surface=platform cannot be used for instance/classic operations")
		}
	}
	return nil
}

func pickAuth(cfg *Config, tenantName string, tenant TenantConfig, candidates []string, op OpClass) (string, AuthEntry, string, error) {
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
		surface := surfaceForOp(op, auth, tenant)
		if err := checkAuthForOp(auth, tenant, op, surface); err != nil {
			errs = append(errs, name+": "+err.Error())
			continue
		}
		// Also verify composed credential validates.
		cred := ComposeCredential(tenant, auth, surface)
		if err := cred.Validate(); err != nil {
			errs = append(errs, name+": "+err.Error())
			continue
		}
		s := scoreAuth(auth, surface, op)
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

func scoreAuth(auth AuthEntry, surface string, op OpClass) int {
	score := 0
	access := auth.EffectiveAccess()
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
		if surf == SurfacePlatform || surf == SurfaceAny {
			score += 100
		}
		if surface == SurfacePlatform {
			score += 20
		}
		if op == OpPlatformWrite && access == AccessFull {
			score += 30
		}
	case OpClassic, OpSchema:
		if surf == SurfaceInstance || surf == SurfaceAny {
			score += 100
		}
		if surface == SurfaceInstance {
			score += 20
		}
	case OpPro, OpAny:
		if access == AccessFull {
			score += 60
		} else {
			score += 40
		}
		if surf == SurfaceAny {
			score += 10
		}
	}
	return score
}

// ResolveSessionForCred is a convenience used by the HTTP stack: credName is an
// optional auth pin (Starlark cred= / MCP cred). Empty uses active selection.
func ResolveSessionForCred(credName string, op OpClass) (*Session, error) {
	if activeConfig == nil {
		return nil, fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	return activeConfig.ResolveSession("", credName, op)
}
