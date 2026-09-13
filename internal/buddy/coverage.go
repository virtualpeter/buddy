package buddy

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// SecretPresence is a presence-only classification (not a live Jamf check).
const (
	SecretPresenceMissing = "missing"
	SecretPresenceStored  = "stored"
	SecretPresenceInline  = "inline"
	SecretPresenceNA      = "n/a"
	// SecretPresenceLocked means vault-backed presence was not checked (no unlock).
	SecretPresenceLocked = "locked"
)

// CoverageOpts controls BuildCoverageReport / AuthSecretPresence vault access.
type CoverageOpts struct {
	// UnlockSecrets opens the vault to classify SECRET as stored/missing.
	// When false (default), vault-backed auths report SecretPresenceLocked
	// without a Touch ID / Keychain prompt.
	UnlockSecrets bool
}

// CoverageAuth is one auth covering a tenant×surface cell.
type CoverageAuth struct {
	Name   string `json:"name"`
	Secret string `json:"secret"` // missing | stored | inline | n/a | locked
	Live   string `json:"live"`   // never | ok | token-ok | token-fail | secret-missing
}

// CoverageCell is one context identity (instance or platform) and the auths that can use it.
type CoverageCell struct {
	Context     string         `json:"context"`
	Tenant      string         `json:"tenant"` // dual-read alias of context
	Org         string         `json:"org,omitempty"`
	Environment string         `json:"environment,omitempty"` // parent environment workplace name
	Kind        string         `json:"kind,omitempty"`
	Surface     string         `json:"surface"`
	Gap         bool           `json:"gap"`
	GapReason   string         `json:"gap_reason,omitempty"`
	Auths       []CoverageAuth `json:"auths,omitempty"`
}

// CoverageReport is the cred-status view of config + store + last live check.
type CoverageReport struct {
	Cells         []CoverageCell `json:"cells"`
	UnlinkedAuths []string       `json:"unlinked_auths,omitempty"`
	ChannelHidden []string       `json:"channel_hidden,omitempty"` // "auth (hidden from mcp, ide)"
	HasGaps       bool           `json:"has_gaps"`
}

// AuthCoversSurface reports whether auth may be used on surface for this tenant identity.
func AuthCoversSurface(auth AuthEntry, surface string) bool {
	surf := auth.EffectiveSurface()
	switch surface {
	case SurfaceInstance:
		return surf == SurfaceInstance
	case SurfacePlatform:
		return auth.EffectiveSurface() == SurfacePlatform
	default:
		return false
	}
}

// AuthFitsWorkplace reports whether auth may be used on this workplace.
// Organization-scoped platform auths are org-level only (not environments or
// product tenants). Other scopes keep their YAML links.
func AuthFitsWorkplace(auth AuthEntry, tenant TenantConfig) bool {
	if auth.EffectiveScope() == PlatformScopeOrganization {
		return tenant.IsOrganizationWorkplace()
	}
	return true
}

// AuthCoversCell is coverage/session: right surface and right workplace kind.
func AuthCoversCell(auth AuthEntry, tenant TenantConfig, surface string) bool {
	return AuthCoversSurface(auth, surface) && AuthFitsWorkplace(auth, tenant)
}

// AuthSecretPresence reports whether the auth has a usable secret (presence only).
// When unlock is false, vault-backed secrets are reported as SecretPresenceLocked
// without opening the vault.
func AuthSecretPresence(cfg *Config, authName string, unlock bool) string {
	if cfg == nil {
		return SecretPresenceMissing
	}
	auth, err := cfg.ResolveAuth(authName)
	if err != nil {
		return SecretPresenceMissing
	}
	switch auth.Type {
	case "command":
		if strings.TrimSpace(auth.RunCommand) != "" {
			return SecretPresenceNA
		}
		return SecretPresenceMissing
	case "basic":
		if strings.TrimSpace(auth.Password) != "" {
			return SecretPresenceInline
		}
		if !unlock {
			return SecretPresenceLocked
		}
		if ok, _ := HasCredentialSecret(authName, SecretKindPassword); ok {
			return SecretPresenceStored
		}
		return SecretPresenceMissing
	case AuthTypeOAuth, AuthTypePlatform:
		if strings.TrimSpace(auth.ClientSecret) != "" {
			return SecretPresenceInline
		}
		if !unlock {
			return SecretPresenceLocked
		}
		if ok, _ := HasCredentialSecret(authName, SecretKindClientSecret); ok {
			return SecretPresenceStored
		}
		return SecretPresenceMissing
	default:
		return SecretPresenceMissing
	}
}

func tenantSurfaces(t TenantConfig) []string {
	var out []string
	if t.HasInstanceURL() {
		out = append(out, SurfaceInstance)
	}
	if t.HasPlatform() {
		out = append(out, SurfacePlatform)
	}
	if len(out) == 0 {
		// Invalid tenant, but still show a row so status is honest.
		out = append(out, SurfaceInstance)
	}
	return out
}

func loadCapsForCell(authName, tenantName, surface string, tenant TenantConfig) *CapsRecord {
	tid := ""
	if surface == SurfacePlatform {
		tid = tenant.EffectiveTenantID()
	}
	return LoadCaps(authName, tenantName, surface, tid)
}

// BuildCoverageReport walks tenants × surfaces using config and caps cache.
// It does not open the vault; SECRET is locked for vault-backed auths.
// Use BuildCoverageReportWith to classify stored/missing.
func BuildCoverageReport(cfg *Config) CoverageReport {
	return BuildCoverageReportWith(cfg, CoverageOpts{})
}

// BuildCoverageReportWith is BuildCoverageReport with optional vault unlock for SECRET.
func BuildCoverageReportWith(cfg *Config, opts CoverageOpts) CoverageReport {
	rep := CoverageReport{}
	if cfg == nil {
		return rep
	}
	tree := WorkplaceTree(cfg.Tenants)
	for _, row := range tree {
		tn := row.Name
		tenant := cfg.Tenants[tn]
		linked := OrderNamesByWorkplaceTree(cfg.Tenants, cfg.AuthsForTenant(tn))
		for _, surface := range tenantSurfaces(tenant) {
			cell := CoverageCell{
				Context:     tn,
				Tenant:      tn,
				Org:         row.Org,
				Environment: row.Environment,
				Kind:        row.Kind,
				Surface:     surface,
			}
			for _, name := range linked {
				auth, err := cfg.ResolveAuth(name)
				if err != nil {
					continue
				}
				if !AuthCoversCell(auth, tenant, surface) {
					continue
				}
				live := LiveNever
				if rec := loadCapsForCell(name, tn, surface, tenant); rec != nil {
					live = liveStatusFromCaps(rec)
				}
				cell.Auths = append(cell.Auths, CoverageAuth{
					Name:   name,
					Secret: AuthSecretPresence(cfg, name, opts.UnlockSecrets),
					Live:   live,
				})
			}
			if len(cell.Auths) == 0 {
				cell.Gap = true
				cell.GapReason = coverageNoAuthReason(tenant, surface)
			} else {
				usable := false
				for _, a := range cell.Auths {
					// locked / stored / inline / n/a count as usable; only missing is a secret gap
					if a.Secret != SecretPresenceMissing {
						usable = true
						break
					}
				}
				if !usable {
					cell.Gap = true
					cell.GapReason = "no secret"
				}
			}
			if cell.Gap {
				rep.HasGaps = true
			}
			rep.Cells = append(rep.Cells, cell)
		}
	}

	anames := make([]string, 0, len(cfg.Auths))
	for n := range cfg.Auths {
		anames = append(anames, n)
	}
	sort.Strings(anames)
	for _, name := range anames {
		a := cfg.Auths[name]
		if len(a.Tenants) == 0 {
			rep.UnlinkedAuths = append(rep.UnlinkedAuths, name)
		}
		channels := a.normalizedChannels()
		if len(channels) == 0 {
			continue
		}
		var hidden []string
		for _, ch := range AllBuddyChannels() {
			if !a.ChannelAllowed(ch) {
				hidden = append(hidden, ch)
			}
		}
		if len(hidden) > 0 {
			rep.ChannelHidden = append(rep.ChannelHidden, fmt.Sprintf("%s (hidden from %s)", name, strings.Join(hidden, ", ")))
		}
	}
	return rep
}

func coverageNoAuthReason(tenant TenantConfig, surface string) string {
	if surface != SurfacePlatform {
		return "no auth"
	}
	switch tenant.WorkplaceKind() {
	case "organization":
		return "no platform auth (scope=organization)"
	case "environment":
		return "no platform auth (scope=environment)"
	case "tenant":
		return "no platform auth (scope=tenant)"
	default:
		return "no auth"
	}
}

// FormatCoverageReport prints the workplace KIND × SURFACE matrix for buddy cred status.
func FormatCoverageReport(rep CoverageReport, w io.Writer) {
	if len(rep.Cells) == 0 && len(rep.UnlinkedAuths) == 0 {
		fmt.Fprintln(w, "No workplaces configured. Use 'buddy setup' or 'buddy cred add' (see 'buddy setup help').")
		return
	}
	fmt.Fprintf(w, "%-14s  %-16s  %-16s  %-15s  %-10s  %-20s  %-8s  %-14s  %s\n", "ORG", "ENVIRONMENT", "CONTEXT", "KIND", "SURFACE", "AUTH", "SECRET", "LIVE", "NOTES")
	fmt.Fprintf(w, "%-14s  %-16s  %-16s  %-15s  %-10s  %-20s  %-8s  %-14s  %s\n",
		"--------------", "----------------", "----------------", "---------------", "----------", "--------------------", "--------", "--------------", "-----")
	for _, cell := range rep.Cells {
		kind := cell.Kind
		if kind == "" {
			kind = "-"
		}
		org, env := cell.Org, cell.Environment
		if len(cell.Auths) == 0 {
			note := ""
			if cell.Gap {
				note = "GAP (" + cell.GapReason + ")"
			}
			fmt.Fprintf(w, "%-14s  %-16s  %-16s  %-15s  %-10s  %-20s  %-8s  %-14s  %s\n",
				org, env, cell.Tenant, kind, cell.Surface, "—", "—", LiveNever, note)
			continue
		}
		for i, a := range cell.Auths {
			note := ""
			if i == 0 && cell.Gap {
				note = "GAP (" + cell.GapReason + ")"
			}
			live := a.Live
			if live == "" {
				live = LiveNever
			}
			fmt.Fprintf(w, "%-14s  %-16s  %-16s  %-15s  %-10s  %-20s  %-8s  %-14s  %s\n",
				org, env, cell.Tenant, kind, cell.Surface, a.Name, a.Secret, live, note)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Rows are org → environment → product-tenant. SURFACE is instance vs Platform gateway (not Account scope).")
	if len(rep.UnlinkedAuths) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "UNLINKED AUTHS (no contexts:)")
		for _, n := range rep.UnlinkedAuths {
			fmt.Fprintf(w, "  %s\n", n)
		}
	}
	if len(rep.ChannelHidden) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "CHANNEL RESTRICTIONS")
		for _, n := range rep.ChannelHidden {
			fmt.Fprintf(w, "  %s\n", n)
		}
	}
	if rep.HasGaps {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Gaps: add an auth with 'buddy cred add' or 'buddy setup secrets'; prove secrets with 'buddy auth test'.")
	}
}

// CoverageTestCells is every covering auth × tenant × surface for auth test --all.
type CoverageTestCell struct {
	Tenant  string
	Surface string
	Auth    string
}

func CoverageTestCells(cfg *Config) []CoverageTestCell {
	rep := BuildCoverageReport(cfg)
	var out []CoverageTestCell
	for _, cell := range rep.Cells {
		for _, a := range cell.Auths {
			out = append(out, CoverageTestCell{Tenant: cell.Tenant, Surface: cell.Surface, Auth: a.Name})
		}
	}
	return out
}
