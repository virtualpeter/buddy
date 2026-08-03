package script

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/buddy/secrets"
)

// mcpGuarded is true when the process channel is MCP (lean tools + buddyscript).
func mcpGuarded() bool {
	return buddy.ActiveChannel() == buddy.ChannelMCP
}

// jamfAuthTokenPath reports whether path is a Jamf token or keep-alive endpoint
// that must not be callable from MCP (agents must not exfiltrate tokens).
func jamfAuthTokenPath(path string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return false
	}
	if u, err := url.Parse(p); err == nil && u.Path != "" {
		p = u.Path
	}
	p = strings.ToLower(p)
	// Strip query.
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	switch {
	case strings.Contains(p, "/api/v1/auth/token"),
		strings.Contains(p, "/api/v1/auth/keep-alive"),
		strings.Contains(p, "/api/oauth/token"),
		strings.HasSuffix(p, "/auth/token"),
		strings.HasSuffix(p, "/oauth2/token"),
		strings.Contains(p, "/oauth2/token"):
		return true
	default:
		return false
	}
}

func errMCPJamfTokenPath() error {
	return fmt.Errorf("MCP channel blocks Jamf token/keep-alive paths (use buddy_info / CLI buddy auth test)")
}

func errMCPSecretPath() error {
	return fmt.Errorf("MCP channel blocks reading buddy config, secrets, and token cache files")
}

// resolvePathAbs cleans, Abs, and EvalSymlinks when possible (macOS /var → /private/var).
func resolvePathAbs(path string) string {
	cleaned, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		cleaned = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved
	}
	// Parent may exist even if the leaf file does not yet.
	dir, base := filepath.Dir(cleaned), filepath.Base(cleaned)
	if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Join(resolvedDir, base)
	}
	return cleaned
}

// buddySecretPath is true when path resolves under buddy's config/secrets/tokens
// (or the file secret-store directory).
func buddySecretPath(path string) bool {
	cleaned := resolvePathAbs(path)

	if cfgDir, err := secrets.ConfigDir(); err == nil {
		cfgDir = resolvePathAbs(cfgDir)
		if underDir(cleaned, cfgDir) {
			rel, relErr := filepath.Rel(cfgDir, cleaned)
			if relErr == nil {
				rel = filepath.ToSlash(rel)
				if rel == "config.yaml" ||
					strings.HasPrefix(rel, "secrets/") ||
					strings.HasPrefix(rel, "tokens/") {
					return true
				}
			}
		}
	}

	if secDir, err := secrets.SecretsDir(); err == nil {
		secDir = resolvePathAbs(secDir)
		if underDir(cleaned, secDir) {
			return true
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		legacy := resolvePathAbs(filepath.Join(home, ".config", "buddy"))
		if underDir(cleaned, legacy) {
			rel, relErr := filepath.Rel(legacy, cleaned)
			if relErr == nil {
				rel = filepath.ToSlash(rel)
				if rel == "config.yaml" || strings.HasPrefix(rel, "secrets/") || strings.HasPrefix(rel, "tokens/") {
					return true
				}
			}
		}
	}
	return false
}

func underDir(path, dir string) bool {
	if dir == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
