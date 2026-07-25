package mcp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"buddy/internal/buddy"
)

// MCPConfigTarget is a well-known host config file that may list MCP servers.
type MCPConfigTarget struct {
	Host string // e.g. "cursor", "claude-desktop"
	Path string // absolute path
}

// WellKnownMCPConfigTargets returns absolute paths for common MCP host configs.
// Paths are always listed (even if the file does not exist yet).
func WellKnownMCPConfigTargets() []MCPConfigTarget {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "~"
	}
	var out []MCPConfigTarget
	out = append(out, MCPConfigTarget{
		Host: "cursor (user)",
		Path: filepath.Join(home, ".cursor", "mcp.json"),
	})
	// Project-local Cursor config relative to cwd.
	if cwd, err := os.Getwd(); err == nil {
		out = append(out, MCPConfigTarget{
			Host: "cursor (project)",
			Path: filepath.Join(cwd, ".cursor", "mcp.json"),
		})
	}
	switch runtime.GOOS {
	case "darwin":
		out = append(out, MCPConfigTarget{
			Host: "claude-desktop",
			Path: filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		})
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		out = append(out, MCPConfigTarget{
			Host: "claude-desktop",
			Path: filepath.Join(appData, "Claude", "claude_desktop_config.json"),
		})
	default: // linux and others
		out = append(out, MCPConfigTarget{
			Host: "claude-desktop",
			Path: filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"),
		})
	}
	return out
}

// BuddyMCPServerEntry is the mcpServers.buddy object written into host configs.
func BuddyMCPServerEntry(buddyPath string) map[string]any {
	if buddyPath == "" {
		buddyPath = "buddy"
	}
	return map[string]any{
		"command": buddyPath,
		"args":    []any{"mcp"},
	}
}

// ResolveBuddyExecutable returns an absolute path to the running buddy binary when possible.
func ResolveBuddyExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return filepath.Clean(exe), nil
	}
	return resolved, nil
}

// MCPConfigStatus describes one config file relative to buddy MCP setup.
type MCPConfigStatus struct {
	Host      string
	Path      string
	Exists    bool
	HasBuddy  bool
	BuddyPath string // command currently configured, if any
	ParseErr  string
}

// InspectMCPConfig reports whether path exists and already has a buddy server entry.
func InspectMCPConfig(path string) MCPConfigStatus {
	st := MCPConfigStatus{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st
		}
		st.ParseErr = err.Error()
		return st
	}
	st.Exists = true
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		st.ParseErr = err.Error()
		return st
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		return st
	}
	entry, ok := servers["buddy"]
	if !ok {
		return st
	}
	st.HasBuddy = true
	if m, ok := entry.(map[string]any); ok {
		if cmd, ok := m["command"].(string); ok {
			st.BuddyPath = cmd
		}
	}
	return st
}

// MergeBuddyMCPConfig reads path (or starts empty), sets mcpServers.buddy, and returns pretty JSON.
func MergeBuddyMCPConfig(path, buddyPath string) ([]byte, error) {
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["buddy"] = BuddyMCPServerEntry(buddyPath)
	root["mcpServers"] = servers
	return json.MarshalIndent(root, "", "  ")
}

// WriteBuddyMCPConfig merges and writes buddy into path (creates parent dirs as needed).
func WriteBuddyMCPConfig(path, buddyPath string) error {
	out, err := MergeBuddyMCPConfig(path, buddyPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SetupOpts controls buddy mcp setup.
type SetupOpts struct {
	Write  bool // merge buddy into selected configs
	Create bool // create missing files when writing (default: only update existing)
	Path   string
	Host   string // filter: cursor, claude-desktop, or empty = all
}

func filterTargets(opts SetupOpts) []MCPConfigTarget {
	targets := WellKnownMCPConfigTargets()
	if opts.Path != "" {
		return []MCPConfigTarget{{Host: "custom", Path: opts.Path}}
	}
	host := strings.ToLower(strings.TrimSpace(opts.Host))
	if host == "" {
		return targets
	}
	var out []MCPConfigTarget
	for _, t := range targets {
		h := strings.ToLower(t.Host)
		switch host {
		case "cursor":
			if strings.HasPrefix(h, "cursor") {
				out = append(out, t)
			}
		case "claude", "claude-desktop":
			if strings.Contains(h, "claude") {
				out = append(out, t)
			}
		default:
			if h == host || strings.HasPrefix(h, host) {
				out = append(out, t)
			}
		}
	}
	return out
}

// SetupMCP discovers well-known MCP host configs and optionally writes a buddy entry.
func SetupMCP(opts SetupOpts, buddyOpts buddy.Opts, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	buddyPath, err := ResolveBuddyExecutable()
	if err != nil {
		buddyPath = "buddy"
	}

	targets := filterTargets(opts)
	if len(targets) == 0 {
		return fmt.Errorf("no MCP config targets matched")
	}

	fmt.Println("Buddy MCP server entry (use absolute command path):")
	snippet, _ := json.MarshalIndent(map[string]any{
		"mcpServers": map[string]any{
			"buddy": BuddyMCPServerEntry(buddyPath),
		},
	}, "", "  ")
	fmt.Println(string(snippet))
	fmt.Println()
	fmt.Println("Well-known MCP host configs:")
	fmt.Printf("%-18s  %-8s  %-10s  %s\n", "Host", "Exists", "Has buddy", "Path")
	fmt.Printf("%-18s  %-8s  %-10s  %s\n", strings.Repeat("-", 18), strings.Repeat("-", 8), strings.Repeat("-", 10), strings.Repeat("-", 40))

	var writePaths []string
	for _, t := range targets {
		st := InspectMCPConfig(t.Path)
		exists := "no"
		if st.Exists {
			exists = "yes"
		}
		has := "no"
		if st.HasBuddy {
			has = "yes"
			if st.BuddyPath != "" && st.BuddyPath != buddyPath {
				has = "yes*"
			}
		}
		if st.ParseErr != "" {
			has = "error"
		}
		fmt.Printf("%-18s  %-8s  %-10s  %s\n", t.Host, exists, has, t.Path)
		if st.ParseErr != "" {
			fmt.Printf("  parse error: %s\n", st.ParseErr)
		}
		if opts.Write {
			if st.Exists || opts.Create || opts.Path != "" {
				writePaths = append(writePaths, t.Path)
			}
		}
	}

	if !opts.Write {
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  yes* means a buddy entry exists but command path differs from this binary.")
		fmt.Println("  Run 'buddy mcp setup --write' to merge the buddy entry into existing configs.")
		fmt.Println("  Add --create to also create missing well-known files (e.g. ~/.cursor/mcp.json).")
		fmt.Println("  Reload / restart the AI host after changing MCP config.")
		return nil
	}

	if len(writePaths) == 0 {
		return fmt.Errorf("no config files to write (use --create to create missing well-known paths, or --path=)")
	}

	fmt.Println()
	for _, path := range writePaths {
		msg := fmt.Sprintf("Write buddy MCP entry to %s?", path)
		if !buddyOpts.SkipConfirm {
			if !buddy.ConfirmPrompt(msg, logger) {
				fmt.Printf("Skipped %s\n", path)
				continue
			}
		} else {
			fmt.Println(msg, "yes (-y)")
		}
		if err := WriteBuddyMCPConfig(path, buddyPath); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("Updated %s\n", path)
	}
	fmt.Println("Reload or restart the AI host so it picks up the MCP server.")
	return nil
}
