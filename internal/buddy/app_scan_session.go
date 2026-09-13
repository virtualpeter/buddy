package buddy

import (
	"log/slog"
	"strings"
)

// pickAppPolicyIndexSession relocates an environment workplace onto a Jamf Pro
// product-tenant child that can serve undocumented GET /api/v1/policies
// (instance URL + instance auth). buddy policy list stays on the environment.
func pickAppPolicyIndexSession(credName string) (tenant, auth string, ok bool) {
	cfg := GetActiveConfig()
	if cfg == nil {
		return "", "", false
	}
	sess, err := ResolveSessionForCred(credName, OpClassic)
	if err != nil {
		return "", "", false
	}
	if sess.Tenant.WorkplaceKind() != "environment" {
		return "", "", false
	}
	pin := strings.TrimSpace(credName)
	for _, name := range ProductTenantsUnderEnvironment(cfg.Tenants, sess.TenantName) {
		child := cfg.Tenants[name]
		if !child.HasInstanceURL() {
			continue
		}
		childSess, err := cfg.ResolveSession(name, pin, OpProUI)
		if err != nil && pin != "" {
			childSess, err = cfg.ResolveSession(name, "", OpProUI)
		}
		if err != nil {
			continue
		}
		return childSess.TenantName, childSess.AuthName, true
	}
	return "", "", false
}

// beginAppPolicyIndexSession applies pickAppPolicyIndexSession for app scan and
// the policy payload cache. restore is always non-nil.
func beginAppPolicyIndexSession(credName string, logger *slog.Logger) (restore func(), pin, indexContext string) {
	pin = credName
	restore = func() {}
	tenant, auth, ok := pickAppPolicyIndexSession(credName)
	if !ok {
		return restore, pin, ""
	}
	r, err := BeginSessionOverrides(SessionOverrides{
		Tenant:  tenant,
		Auth:    auth,
		Surface: SurfaceInstance,
	})
	if err != nil {
		return restore, pin, ""
	}
	if logger != nil {
		logger.Debug("app policy index using product tenant", "context", tenant, "auth", auth)
	}
	return r, auth, tenant
}
