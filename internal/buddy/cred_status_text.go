package buddy

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"buddy/internal/buddy/agent"
	"buddy/internal/buddy/secrets"
)

// WriteCredStatusHeader prints where this process keeps config and secrets.
// It does not unlock the vault.
func WriteCredStatusHeader(w io.Writer, cfg *Config) error {
	text, err := credStatusHeader(cfg)
	if err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	_, err = io.WriteString(w, text)
	return err
}

func credStatusHeader(cfg *Config) (string, error) {
	var b strings.Builder
	if err := writeConfigLines(&b, cfg); err != nil {
		return "", err
	}
	if cfg.UseAgent() {
		if err := writeAgentSecretLines(&b); err != nil {
			return "", err
		}
	}
	if !cfg.UseAgent() || cfgHasLocalAuth(cfg) {
		if err := writeLocalSecretLines(&b, cfg.UseAgent()); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func writeConfigLines(b *strings.Builder, cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if cfg.UseAgent() || cfg.HasAgentConfig() {
		agentPath, err := agent.ConfigPath()
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "config: %s%s\n", agentPath, mode0600())
		fmt.Fprintf(b, "user config: %s%s\n", path, configFileNote(path))
		return nil
	}
	fmt.Fprintf(b, "config: %s%s\n", path, configFileNote(path))
	return nil
}

func writeAgentSecretLines(b *strings.Builder) error {
	sock, err := agent.SocketPath()
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "secrets: Buddy Agent Keychain\n")
	fmt.Fprintf(b, "  service: %s\n", agent.KeychainService)
	fmt.Fprintf(b, "  socket: %s%s\n", sock, mode0600())
	fmt.Fprintf(b, "  The Agent holds those secrets and sends tokens to buddy.\n")
	return nil
}

func cfgHasLocalAuth(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	for name := range cfg.Auths {
		if !cfg.AuthFromAgent(name) {
			return true
		}
	}
	return false
}

func writeLocalSecretLines(b *strings.Builder, alongsideAgent bool) error {
	if alongsideAgent {
		fmt.Fprintf(b, "auths in config.yaml:\n")
	}
	vault, err := secrets.VaultPath()
	if err != nil {
		return err
	}
	tokens, err := secrets.TokensDir()
	if err != nil {
		return err
	}
	switch secrets.DEKKindName() {
	case "file":
		dek, err := secrets.DekFilePath()
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "secrets: files\n")
		fmt.Fprintf(b, "  vault: %s%s\n", vault, mode0600())
		fmt.Fprintf(b, "  dek: %s%s\n", dek, mode0600())
		if runtime.GOOS != "windows" {
			fmt.Fprintf(b, "  Anyone who can read the dek and the vault can decrypt the secrets.\n")
		}
	case "pass":
		fmt.Fprintf(b, "secrets: pass\n")
		fmt.Fprintf(b, "  vault: %s%s\n", vault, mode0600())
		fmt.Fprintf(b, "  dek: pass show %s\n", secrets.PassDEKEntry())
	case "keychain":
		item, ok := secrets.KeychainDEKItem()
		if !ok {
			fmt.Fprintf(b, "secrets: macOS Keychain (not available on this OS)\n")
			break
		}
		switch item.Access {
		case "framework":
			fmt.Fprintf(b, "secrets: macOS Keychain via Security.framework (signed helper)\n")
			fmt.Fprintf(b, "  vault: %s%s\n", vault, mode0600())
			fmt.Fprintf(b, "  dek: Keychain service %q, account %q\n", item.Service, item.Account)
			fmt.Fprintf(b, "  The data key is presence-gated when this binary is entitled; otherwise LocalAuthentication runs before it is read.\n")
		default:
			fmt.Fprintf(b, "secrets: macOS Keychain via this unsigned binary (security CLI)\n")
			fmt.Fprintf(b, "  vault: %s%s\n", vault, mode0600())
			fmt.Fprintf(b, "  dek: Keychain service %q, account %q\n", item.Service, item.Account)
			fmt.Fprintf(b, "  The Keychain ACL is on /usr/bin/security, so any process running as you can read the data key.\n")
		}
		if wrap, ok := secrets.KeychainTokenWrapItem(); ok {
			fmt.Fprintf(b, "  tokens: %s (wrap key: Keychain service %q, account %q)\n", tokens, wrap.Service, wrap.Account)
		}
	default:
		fmt.Fprintf(b, "secrets: %s\n", secrets.DEKKindName())
		fmt.Fprintf(b, "  vault: %s%s\n", vault, mode0600())
	}
	if secrets.DEKKindName() != "keychain" {
		fmt.Fprintf(b, "  tokens: %s%s\n", tokens, tokenFilesNote())
	}
	status, detail := secrets.RecoveryStatus()
	if detail != "" {
		fmt.Fprintf(b, "  recovery: %s (%s)\n", status, detail)
	}
	return nil
}

func mode0600() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return " (mode 0600)"
}

func tokenFilesNote() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return " (files mode 0600)"
}

// configFileNote is the mode SaveConfig uses. A missing file is named as not written.
func configFileNote(path string) string {
	missing := false
	if path != "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = true
		}
	}
	switch {
	case missing && runtime.GOOS == "windows":
		return " (not written)"
	case missing:
		return " (not written, mode 0600)"
	default:
		return mode0600()
	}
}
