package buddy

import (
	"fmt"
	"log/slog"
	"strings"
)

// organizationWorkplaceFor returns the organization context that should serve
// Account org APIs (licensing, later SSO) for workplace name. If name is already
// an organization workplace, it is returned. Otherwise TenantConfig.Org is the
// parent. Empty when there is no usable parent.
func organizationWorkplaceFor(cfg *Config, name string, t TenantConfig) string {
	if cfg == nil {
		return ""
	}
	if t.IsOrganizationWorkplace() {
		return name
	}
	org := strings.TrimSpace(t.Org)
	if org == "" {
		return ""
	}
	parent, ok := cfg.Tenants[org]
	if !ok || !parent.IsOrganizationWorkplace() {
		return ""
	}
	return org
}

// selectedWorkplace is the current context: a resolved session when possible,
// otherwise the active/default tenant so an org: parent can still be found
// when the child workplace has no usable auth.
func selectedWorkplace(cfg *Config, credName string) (string, TenantConfig, error) {
	sess, err := ResolveSessionForCred(credName, OpAny)
	if err == nil {
		return sess.TenantName, sess.Tenant, nil
	}
	if strings.TrimSpace(credName) != "" {
		return "", TenantConfig{}, err
	}
	name, _ := ActiveSelection()
	if name == "" {
		name = cfg.DefaultTenant(ActiveChannel())
	}
	t, rerr := cfg.ResolveTenant(name)
	if rerr != nil {
		return "", TenantConfig{}, err
	}
	return name, t, nil
}

// pickOrgAccountSession relocates an environment or product-tenant workplace
// onto its parent organization context for Account org APIs. Organization-scoped
// platform auths are picked on the parent; environment/tenant pins are not reused.
func pickOrgAccountSession(credName string) (tenant, auth string, err error) {
	cfg := GetActiveConfig()
	if cfg == nil {
		return "", "", fmt.Errorf("internal error: config not loaded (SetActiveConfig not called)")
	}
	name, t, err := selectedWorkplace(cfg, credName)
	if err != nil {
		return "", "", err
	}
	orgName := organizationWorkplaceFor(cfg, name, t)
	if orgName == "" {
		return "", "", fmt.Errorf("Account licensing is organization-scoped; context %q has no organization parent (set org: or use --context on an organization workplace)", name)
	}
	orgTenant, err := cfg.ResolveTenant(orgName)
	if err != nil {
		return "", "", err
	}
	pin := strings.TrimSpace(credName)
	if pin != "" {
		if a, aerr := cfg.ResolveAuth(pin); aerr == nil && a.EffectiveScope() == PlatformScopeOrganization {
			orgSess, rerr := cfg.ResolveSession(orgName, pin, OpPlatform)
			if rerr == nil {
				return orgSess.TenantName, orgSess.AuthName, nil
			}
		}
	}
	// autoPickSession ignores the process auth pin (ResolveSession("") would
	// reuse an environment-scoped pin from --auth / the default context).
	orgSess, err := cfg.autoPickSession(orgName, orgTenant, OpPlatform, ActiveChannel())
	if err != nil {
		return "", "", fmt.Errorf("Account licensing requires a platform auth on organization workplace %q: %w", orgName, err)
	}
	return orgSess.TenantName, orgSess.AuthName, nil
}

// beginOrgAccountSession applies pickOrgAccountSession for Account org APIs.
// restore is always non-nil.
func beginOrgAccountSession(credName string, logger *slog.Logger) (restore func(), pin string, err error) {
	restore = func() {}
	tenant, auth, err := pickOrgAccountSession(credName)
	if err != nil {
		return restore, "", err
	}
	r, err := BeginSessionOverrides(SessionOverrides{
		Tenant:  tenant,
		Auth:    auth,
		Surface: SurfacePlatform,
	})
	if err != nil {
		return restore, "", err
	}
	if logger != nil {
		logger.Debug("account api using organization workplace", "context", tenant, "auth", auth)
	}
	return r, auth, nil
}
