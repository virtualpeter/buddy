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
)

// CoverageAuth is one auth covering a tenant×surface cell.
type CoverageAuth struct {
	Name   string
	Secret string // missing | stored | inline | n/a
	Live   string // never | ok | token-ok | token-fail | secret-missing
}

// CoverageCell is one tenant identity (instance or platform) and the auths that can use it.
type CoverageCell struct {
	Tenant    string
	Surface   string
	Gap       bool
	GapReason string
	Auths     []CoverageAuth
}

// CoverageReport is the cred-status view of config + store + last live check.
type CoverageReport struct {
	Cells         []CoverageCell
	UnlinkedAuths []string
	ChannelHidden []string // "auth (hidden from mcp, ide)"
	HasGaps       bool
}

// AuthCoversSurface reports whether auth may be used on surface for this tenant identity.
func AuthCoversSurface(auth AuthEntry, surface string) bool {
	surf := auth.EffectiveSurface()
	switch surface {
	case SurfaceInstance:
		return surf == SurfaceInstance || surf == SurfaceAny
	case SurfacePlatform:
		if auth.Type != "oauth" {
			return false
		}
		return surf == SurfacePlatform || surf == SurfaceAny
	default:
		return false
	}
}

// AuthSecretPresence reports whether the auth has a usable secret (presence only).
func AuthSecretPresence(cfg *Config, authName string) string {
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
		if ok, _ := HasCredentialSecret(authName, SecretKindPassword); ok {
			return SecretPresenceStored
		}
		return SecretPresenceMissing
	case "oauth":
		if strings.TrimSpace(auth.ClientSecret) != "" {
			return SecretPresenceInline
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
		out = append(out, SurfaceAny)
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

// BuildCoverageReport walks tenants × surfaces using config, the secret store, and caps cache.
func BuildCoverageReport(cfg *Config) CoverageReport {
	rep := CoverageReport{}
	if cfg == nil {
		return rep
	}
	tnames := make([]string, 0, len(cfg.Tenants))
	for n := range cfg.Tenants {
		tnames = append(tnames, n)
	}
	sort.Strings(tnames)

	for _, tn := range tnames {
		tenant := cfg.Tenants[tn]
		linked := cfg.AuthsForTenant(tn)
		sort.Strings(linked)
		for _, surface := range tenantSurfaces(tenant) {
			cell := CoverageCell{Tenant: tn, Surface: surface}
			for _, name := range linked {
				auth, err := cfg.ResolveAuth(name)
				if err != nil {
					continue
				}
				if !AuthCoversSurface(auth, surface) {
					continue
				}
				live := LiveNever
				if rec := loadCapsForCell(name, tn, surface, tenant); rec != nil {
					live = liveStatusFromCaps(rec)
				}
				cell.Auths = append(cell.Auths, CoverageAuth{
					Name:   name,
					Secret: AuthSecretPresence(cfg, name),
					Live:   live,
				})
			}
			if len(cell.Auths) == 0 {
				cell.Gap = true
				cell.GapReason = "no auth"
			} else {
				usable := false
				for _, a := range cell.Auths {
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

// FormatCoverageReport prints the tenant × surface matrix for buddy cred status.
func FormatCoverageReport(rep CoverageReport, w io.Writer) {
	if len(rep.Cells) == 0 && len(rep.UnlinkedAuths) == 0 {
		fmt.Fprintln(w, "No tenants configured. Use 'buddy cred add' or 'buddy cred set tenant' (see 'buddy cred help').")
		return
	}
	fmt.Fprintf(w, "%-16s  %-10s  %-20s  %-8s  %-14s  %s\n", "TENANT", "SURFACE", "AUTH", "SECRET", "LIVE", "NOTES")
	fmt.Fprintf(w, "%-16s  %-10s  %-20s  %-8s  %-14s  %s\n",
		"----------------", "----------", "--------------------", "--------", "--------------", "-----")
	for _, cell := range rep.Cells {
		if len(cell.Auths) == 0 {
			note := ""
			if cell.Gap {
				note = "GAP (" + cell.GapReason + ")"
			}
			fmt.Fprintf(w, "%-16s  %-10s  %-20s  %-8s  %-14s  %s\n",
				cell.Tenant, cell.Surface, "—", "—", LiveNever, note)
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
			fmt.Fprintf(w, "%-16s  %-10s  %-20s  %-8s  %-14s  %s\n",
				cell.Tenant, cell.Surface, a.Name, a.Secret, live, note)
		}
	}
	if len(rep.UnlinkedAuths) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "UNLINKED AUTHS (no tenants:)")
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
