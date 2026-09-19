package buddy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buddy/internal/buddy/agent"
	"buddy/internal/buddy/secrets"
)

// serveAgentConfig makes LoadConfig see an Agent document without a socket.
func serveAgentConfig(t *testing.T, doc string) {
	t.Helper()
	prev := agentConfigFetch
	agentConfigFetch = func(context.Context) (json.RawMessage, error) {
		return json.RawMessage(doc), nil
	}
	t.Cleanup(func() { agentConfigFetch = prev })
	t.Setenv(agent.EnvDisable, "")
	t.Setenv(agent.EnvSocket, filepath.Join(t.TempDir(), "agent.sock"))
}

const agentDoc = `{
  "context": {"default": "sandbox"},
  "contexts": {"sandbox": {"url": "https://agent.example.com"}},
  "auths": {"sandbox-agent": {"type": "oauth", "clientId": "agent-cid", "contexts": ["sandbox"]}}
}`

func TestLoadConfig_AgentLayerBeneathUserFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "features:\n  use_agent: true\ncontexts:\n  local:\n    url: https://local.example.com\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.UseAgent() {
		t.Fatal("features.use_agent should be on")
	}
	if !cfg.HasAgentConfig() {
		t.Fatal("expected an agent layer")
	}
	if cfg.Tenants["sandbox"].URL != "https://agent.example.com" {
		t.Fatalf("agent context missing: %+v", cfg.Tenants)
	}
	if cfg.Tenants["local"].URL != "https://local.example.com" {
		t.Fatalf("user context missing: %+v", cfg.Tenants)
	}
	if !cfg.AuthFromAgent("sandbox-agent") || cfg.AuthFromAgent("nope") {
		t.Fatal("AuthFromAgent is wrong")
	}
	if !cfg.ContextFromAgent("sandbox") || cfg.ContextFromAgent("local") {
		t.Fatal("ContextFromAgent is wrong")
	}
	if sum := cfg.AgentConfigSummary(); !strings.Contains(sum, "sandbox-agent") {
		t.Fatalf("summary = %q", sum)
	}

	// Saving must not bake the Agent's entries into config.yaml.
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	if strings.Contains(saved, "agent.example.com") || strings.Contains(saved, "sandbox-agent") {
		t.Fatalf("agent layer was persisted:\n%s", saved)
	}
	if !strings.Contains(saved, "local.example.com") {
		t.Fatalf("user entry lost:\n%s", saved)
	}
	if !strings.Contains(saved, "use_agent: true") {
		t.Fatalf("use_agent flag lost:\n%s", saved)
	}
}

func TestLoadConfig_UserOverridesAgentEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "features:\n  use_agent: true\ncontexts:\n  sandbox:\n    url: https://override.example.com\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Tenants["sandbox"].URL != "https://override.example.com" {
		t.Fatalf("user file must win: %+v", cfg.Tenants["sandbox"])
	}
}

func TestLoadConfig_AgentAbsentIsFatalWhenRequired(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	prev := agentConfigFetch
	agentConfigFetch = func(context.Context) (json.RawMessage, error) {
		return nil, agent.ErrAbsent
	}
	t.Cleanup(func() { agentConfigFetch = prev })
	t.Setenv(agent.EnvDisable, "")
	t.Setenv(agent.EnvSocket, filepath.Join(dir, "agent.sock"))

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("features:\n  use_agent: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("err = %v, want the not-running abort", err)
	}

	// The kill switch is the same abort, not a silent fallback.
	t.Setenv(agent.EnvDisable, "1")
	_, err = LoadConfig()
	if err == nil || !strings.Contains(err.Error(), agent.EnvDisable) {
		t.Fatalf("err = %v, want the BUDDY_NO_AGENT abort", err)
	}
}

func TestSetUseAgentWritesFlagWithoutAgentEntries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := SetUseAgent(cfg, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	path := filepath.Join(dir, "buddy", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	if !strings.Contains(saved, "use_agent: true") {
		t.Fatalf("enable did not persist:\n%s", saved)
	}
	if strings.Contains(saved, "agent.example.com") || strings.Contains(saved, "sandbox-agent") {
		t.Fatalf("agent layer was persisted:\n%s", saved)
	}

	if err := SetUseAgent(cfg, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "use_agent: false") {
		t.Fatalf("disable did not persist false:\n%s", data)
	}
}

func TestSetForceAgentWritesFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))

	if err := SetForceAgent(&Config{}, true); err != nil {
		t.Fatalf("force: %v", err)
	}
	path := filepath.Join(dir, "buddy", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "force_agent: true") {
		t.Fatalf("force did not persist:\n%s", data)
	}
	if err := SetForceAgent(&Config{}, false); err != nil {
		t.Fatalf("force off: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "force_agent: false") {
		t.Fatalf("force --off did not persist:\n%s", data)
	}
}

func TestSetUseAgentCreatesFileWhenMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))

	if err := SetUseAgent(&Config{}, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "buddy", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	if !strings.Contains(saved, "use_agent: true") {
		t.Fatalf("missing file should get the flag:\n%s", saved)
	}
	if strings.Contains(saved, "context:") && strings.Contains(saved, "default: default") {
		t.Fatalf("must not invent context.default:\n%s", saved)
	}
}

func TestLoadConfig_ForceAgentIgnoresUserYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "features:\n  force_agent: true\ncontexts:\n  local:\n    url: https://local.example.com\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.ForceAgent() || !cfg.UseAgent() {
		t.Fatal("force_agent should require the Agent")
	}
	if cfg.Tenants["sandbox"].URL != "https://agent.example.com" {
		t.Fatalf("agent context missing: %+v", cfg.Tenants)
	}
	if _, ok := cfg.Tenants["local"]; ok {
		t.Fatalf("user context must be ignored: %+v", cfg.Tenants)
	}
}

func TestLoadConfig_ForceAgentFromMDMIgnoresUserYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	plistPath := filepath.Join(dir, "managed.plist")
	writeTestPlist(t, plistPath, map[string]any{
		"features": map[string]any{"force_agent": true},
		"contexts": map[string]any{
			"mdm": map[string]any{"url": "https://mdm.example.com"},
		},
	})
	t.Setenv(EnvManagedPlist, plistPath)
	serveAgentConfig(t, agentDoc)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "features:\n  force_agent: false\ncontexts:\n  local:\n    url: https://local.example.com\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.ForceAgent() {
		t.Fatal("MDM force_agent: true must win over user false")
	}
	if cfg.Tenants["sandbox"].URL != "https://agent.example.com" {
		t.Fatalf("agent context missing: %+v", cfg.Tenants)
	}
	if cfg.Tenants["mdm"].URL != "https://mdm.example.com" {
		t.Fatalf("MDM context missing: %+v", cfg.Tenants)
	}
	if _, ok := cfg.Tenants["local"]; ok {
		t.Fatalf("user context must be ignored: %+v", cfg.Tenants)
	}
}

func TestParseAgentConfigDocumentDropsForceAgent(t *testing.T) {
	cfg, err := parseAgentConfigDocument(json.RawMessage(`{"features":{"force_agent":true,"use_agent":true}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Features.ForceAgent != nil || cfg.Features.UseAgent != nil {
		t.Fatal("stale force_agent/use_agent must be stripped")
	}
}

func TestLoadConfig_NoAgentDialWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	prev := agentConfigFetch
	agentConfigFetch = func(context.Context) (json.RawMessage, error) {
		t.Fatal("the agent must not be dialled when features.use_agent is off")
		return nil, errors.New("unreachable")
	}
	t.Cleanup(func() { agentConfigFetch = prev })

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("contexts:\n  local:\n    url: https://local.example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HasAgentConfig() {
		t.Fatal("no agent layer expected")
	}
}

func TestLoadConfig_MissingConfigDiscoversAgent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.UseAgent() || !cfg.HasAgentConfig() {
		t.Fatal("empty install should use a contactable Agent")
	}
	if cfg.Tenants["sandbox"].URL != "https://agent.example.com" {
		t.Fatalf("agent context missing: %+v", cfg.Tenants)
	}
	if ConfigNeedsSetup(cfg) {
		t.Fatal("agent workplaces should skip first-run setup")
	}
	if _, err := os.Stat(filepath.Join(dir, "buddy", "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("discovering the Agent must leave config.yaml absent: %v", err)
	}
}

func TestLoadConfig_EmptyFileDiscoversAgent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.UseAgent() || ConfigNeedsSetup(cfg) {
		t.Fatal("empty file should use a contactable Agent")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "use_agent") {
		t.Fatalf("must not write features.use_agent:\n%s", data)
	}
}

func TestLoadConfig_EmptyConfigAgentAbsentNotFatal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	prev := agentConfigFetch
	agentConfigFetch = func(context.Context) (json.RawMessage, error) {
		return nil, agent.ErrAbsent
	}
	t.Cleanup(func() { agentConfigFetch = prev })
	t.Setenv(agent.EnvDisable, "")
	t.Setenv(agent.EnvSocket, filepath.Join(dir, "agent.sock"))

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("absent Agent on empty config must not fail: %v", err)
	}
	if cfg.UseAgent() || cfg.HasAgentConfig() {
		t.Fatal("absent Agent must not attach an Agent layer")
	}
	if !ConfigNeedsSetup(cfg) {
		t.Fatal("expected first-run setup")
	}
	if _, err := os.Stat(filepath.Join(dir, "buddy", "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("absent Agent should not create config.yaml: %v", err)
	}
}

func TestLoadConfig_EmptyConfigAgentNeedsConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, `{"channels":{"cli":{"access":"readonly"}}}`)

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "needs config") {
		t.Fatalf("err = %v, want Agent needs config", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "buddy", "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("must not create config.yaml: %v", err)
	}
}

func TestLoadConfig_UseAgentFlagEmptyAgentNeedsConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, `{}`)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("features:\n  use_agent: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "needs config") {
		t.Fatalf("err = %v, want Agent needs config", err)
	}
}

func TestLoadConfig_EmptyConfigUseAgentOffDoesNotDial(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	t.Setenv(EnvUseAgent, "0")
	prev := agentConfigFetch
	agentConfigFetch = func(context.Context) (json.RawMessage, error) {
		t.Fatal("BUDDY_USE_AGENT=0 must not discover the Agent")
		return nil, errors.New("unreachable")
	}
	t.Cleanup(func() { agentConfigFetch = prev })

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.UseAgent() || cfg.HasAgentConfig() {
		t.Fatal("explicit off must keep first-run setup")
	}
}

func TestAgentPruneDropsDuplicateEntries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)
	secrets.SetStore(&mockSecretStore{values: map[string]string{
		"sandbox-agent/client-secret": "local-copy",
	}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Same names as the Agent document, plus one purely local workplace.
	user := "features:\n  use_agent: true\ncontexts:\n" +
		"  sandbox:\n    url: https://stale.example.com\n" +
		"  local:\n    url: https://local.example.com\n" +
		"auths:\n  sandbox-agent:\n    type: oauth\n    clientId: stale-cid\n    contexts: [sandbox]\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	dups := AgentDuplicates(cfg)
	if len(dups) != 2 {
		t.Fatalf("duplicates = %+v", dups)
	}
	for _, d := range dups {
		if !d.Differs || d.Detail == "" {
			t.Fatalf("stale copy should be reported as different: %+v", d)
		}
	}

	res, err := PruneAgentDuplicates(cfg, true)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(res.Auths) != 1 || res.Auths[0] != "sandbox-agent" {
		t.Fatalf("pruned auths = %v", res.Auths)
	}
	if len(res.Secrets) != 1 || res.Secrets[0] != "sandbox-agent" {
		t.Fatalf("deleted secrets = %v", res.Secrets)
	}
	if ok, _ := HasCredentialSecret("sandbox-agent", secrets.KindClientSecret); ok {
		t.Fatal("the local secret copy should be gone")
	}

	// The Agent's values are now the live ones, and the local-only entry stays.
	if cfg.Tenants["sandbox"].URL != "https://agent.example.com" {
		t.Fatalf("agent context should win now: %+v", cfg.Tenants["sandbox"])
	}
	if cfg.Auths["sandbox-agent"].ClientID != "agent-cid" {
		t.Fatalf("agent auth should win now: %+v", cfg.Auths["sandbox-agent"])
	}
	if cfg.Tenants["local"].URL != "https://local.example.com" {
		t.Fatalf("local-only context was dropped: %+v", cfg.Tenants)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	if strings.Contains(saved, "stale.example.com") || strings.Contains(saved, "stale-cid") {
		t.Fatalf("duplicate survived in config.yaml:\n%s", saved)
	}
	if !strings.Contains(saved, "local.example.com") || !strings.Contains(saved, "use_agent: true") {
		t.Fatalf("config.yaml lost user state:\n%s", saved)
	}
	if len(AgentDuplicates(cfg)) != 0 {
		t.Fatalf("still duplicated: %+v", AgentDuplicates(cfg))
	}
}

// The end state the Agent is for: config.yaml shrinks to the feature flag.
func TestAgentPruneFullHandoverLeavesOnlyTheFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentDoc)
	secrets.SetStore(&mockSecretStore{values: map[string]string{}})
	t.Cleanup(func() { secrets.SetStore(nil) })

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "context:\n  default: sandbox\nfeatures:\n  use_agent: true\ncontexts:\n" +
		"  sandbox:\n    url: https://agent.example.com\n" +
		"auths:\n  sandbox-agent:\n    type: oauth\n    clientId: agent-cid\n    contexts: [sandbox]\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// The invented organization node goes too, once its only child is shared.
	if _, err := PruneAgentDuplicates(cfg, false); err != nil {
		t.Fatalf("prune: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	for _, gone := range []string{"agent.example.com", "sandbox-agent", "agent-cid", "orgs:"} {
		if strings.Contains(saved, gone) {
			t.Fatalf("%q should have been handed to the Agent:\n%s", gone, saved)
		}
	}
	if !strings.Contains(saved, "use_agent: true") {
		t.Fatalf("the flag must stay:\n%s", saved)
	}
	// Reloading still sees the Agent's copy, from the Agent alone.
	reloaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Tenants["sandbox"].URL != "https://agent.example.com" {
		t.Fatalf("agent context lost after prune: %+v", reloaded.Tenants)
	}
	if !reloaded.AuthFromAgent("sandbox-agent") {
		t.Fatal("auth should now come from the Agent")
	}
	if len(AgentDuplicates(reloaded)) != 0 {
		t.Fatalf("still duplicated: %+v", AgentDuplicates(reloaded))
	}
}

func TestAgentOwnedEntriesRefuseEdits(t *testing.T) {
	cfg := &Config{
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://agent.example.com"}},
		Auths:   map[string]AuthEntry{"sandbox-agent": {Type: AuthTypeOAuth, ClientID: "cid"}},
		agent: &Config{
			Tenants: map[string]TenantConfig{"sandbox": {URL: "https://agent.example.com"}},
			Auths:   map[string]AuthEntry{"sandbox-agent": {Type: AuthTypeOAuth, ClientID: "cid"}},
		},
	}
	if _, err := SetTenantMetadata(cfg, "sandbox", SetTenantOpts{URL: "https://x", Overwrite: true}); err == nil ||
		!strings.Contains(err.Error(), "Buddy Agent") {
		t.Fatalf("set context err = %v", err)
	}
	if _, err := SetAuthMetadata(cfg, "sandbox-agent", SetAuthMetadataOpts{Type: "oauth", Overwrite: true}); err == nil ||
		!strings.Contains(err.Error(), "Buddy Agent") {
		t.Fatalf("set auth err = %v", err)
	}
	if _, err := DeleteTenantMetadata(cfg, "sandbox"); err == nil || !strings.Contains(err.Error(), "Buddy Agent") {
		t.Fatalf("delete context err = %v", err)
	}
	if err := DeleteAuthMetadata(cfg, "sandbox-agent"); err == nil || !strings.Contains(err.Error(), "Buddy Agent") {
		t.Fatalf("delete auth err = %v", err)
	}
	if err := RenameAuth(cfg, "sandbox-agent", "other"); err == nil || !strings.Contains(err.Error(), "Buddy Agent") {
		t.Fatalf("rename auth err = %v", err)
	}
	cfg.Channels = map[string]ChannelPolicy{ChannelCLI: {Access: AccessReadonly}}
	cfg.agent.Channels = map[string]ChannelPolicy{ChannelCLI: {Access: AccessReadonly}}
	if err := cfg.SetChannelPolicy(ChannelCLI, AccessFull, "", nil, true, false, false, nil, false, nil, false); err == nil ||
		!strings.Contains(err.Error(), "Buddy Agent") {
		t.Fatalf("set channel err = %v", err)
	}
}

func TestCoverageShowsAgentSecret(t *testing.T) {
	cfg := &Config{
		Tenant:  TenantSelection{Default: "sandbox"},
		Tenants: map[string]TenantConfig{"sandbox": {URL: "https://agent.example.com"}},
		Auths: map[string]AuthEntry{"sandbox-agent": {
			Type: AuthTypeOAuth, ClientID: "cid", Tenants: []string{"sandbox"},
		}},
		agent: &Config{Auths: map[string]AuthEntry{"sandbox-agent": {Type: AuthTypeOAuth}}},
	}
	if got := AuthSecretPresence(cfg, "sandbox-agent", true); got != SecretPresenceAgent {
		t.Fatalf("presence = %q, want %q", got, SecretPresenceAgent)
	}
	var out strings.Builder
	FormatCoverageReport(BuildCoverageReport(cfg), &out)
	if !strings.Contains(out.String(), SecretPresenceAgent) {
		t.Fatalf("coverage output:\n%s", out.String())
	}
	snap := BuildConfigSnapshot(cfg)
	if len(snap.Auths) != 1 || !snap.Auths[0].Agent {
		t.Fatalf("snapshot auths = %+v", snap.Auths)
	}
}

func TestParseAgentConfigDocumentDropsSecrets(t *testing.T) {
	doc := `{"contexts":{"a":{"url":"https://a.example.com"}},
	         "auths":{"a-auth":{"type":"oauth","clientId":"cid","client_secret":"nope","password":"nope"}}}`
	cfg, err := parseAgentConfigDocument(json.RawMessage(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	entry := cfg.Auths["a-auth"]
	if entry.ClientID != "cid" {
		t.Fatalf("auth = %+v", entry)
	}
	if entry.ClientSecret != "" || entry.Password != "" {
		t.Fatal("a secret survived the agent document parse")
	}
}

// agentIdentityDoc is the shape AgentConfigExport.identityDocument serves:
// orgs tree, leftover contexts (policy lists the tree cannot carry), channels,
// features, and auths with scope/access/channels/deny/allow.
const agentIdentityDoc = `{
  "context": {"default": "sandbox"},
  "features": {"classic_instance_only": true},
  "channels": {
    "cli": {"access": "readonly", "context": "sandbox", "contexts": ["sandbox"], "deny_builtins": ["computer_erase"]},
    "mcp": {"allow_builtins": ["*"]}
  },
  "orgs": {
    "acme": {
      "environments": {
        "commercial": {
          "id": "11111111-1111-1111-1111-111111111111",
          "region": "us",
          "tenants": {
            "sandbox": {
              "tenantId": "22222222-2222-2222-2222-222222222222",
              "url": "https://agent.example.com"
            }
          }
        }
      }
    }
  },
  "contexts": {
    "sandbox": {
      "jamfpro": {"url": "https://agent.example.com", "tenantId": "22222222-2222-2222-2222-222222222222"},
      "deny_builtins": ["*"],
      "allow_builtins": ["policies"]
    }
  },
  "auths": {
    "sandbox-agent": {
      "type": "platform",
      "clientId": "agent-cid",
      "scope": "tenant",
      "access": "readonly",
      "contexts": ["sandbox"],
      "channels": ["cli", "mcp"],
      "deny_builtins": ["computer_erase"],
      "allow_builtins": ["policies"]
    }
  }
}`

func TestParseAgentConfigDocumentUsesFullIdentity(t *testing.T) {
	cfg, err := parseAgentConfigDocument(json.RawMessage(agentIdentityDoc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Tenant.Default != "sandbox" {
		t.Fatalf("default = %q", cfg.Tenant.Default)
	}
	if cfg.Features.UseAgent != nil {
		t.Fatal("agent document must not carry features.use_agent")
	}
	if cfg.Features.ClassicInstanceOnly == nil || !*cfg.Features.ClassicInstanceOnly {
		t.Fatal("features.classic_instance_only")
	}

	cli := cfg.Channels[ChannelCLI]
	if cli.EffectiveAccess() != AccessReadonly || cli.Tenant != "sandbox" || len(cli.Tenants) != 1 || cli.Tenants[0] != "sandbox" {
		t.Fatalf("channels.cli = %+v", cli)
	}
	if len(cli.DenyBuiltins) != 1 || cli.DenyBuiltins[0] != "computer_erase" {
		t.Fatalf("channels.cli.deny = %v", cli.DenyBuiltins)
	}
	mcp := cfg.Channels[ChannelMCP]
	if len(mcp.AllowBuiltins) != 1 || mcp.AllowBuiltins[0] != "*" {
		t.Fatalf("channels.mcp.allow = %v", mcp.AllowBuiltins)
	}

	sandbox := cfg.Tenants["sandbox"]
	if sandbox.URL != "https://agent.example.com" || sandbox.TenantID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("sandbox workplace = %+v", sandbox)
	}
	if sandbox.EnvironmentID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("sandbox environmentId = %q (org expansion should fill it)", sandbox.EnvironmentID)
	}
	if len(sandbox.DenyBuiltins) != 1 || sandbox.DenyBuiltins[0] != "*" {
		t.Fatalf("sandbox deny = %v", sandbox.DenyBuiltins)
	}
	if len(sandbox.AllowBuiltins) != 1 || sandbox.AllowBuiltins[0] != "policies" {
		t.Fatalf("sandbox allow = %v", sandbox.AllowBuiltins)
	}
	if _, ok := cfg.Orgs["acme"]; !ok {
		t.Fatalf("orgs.acme missing: %+v", cfg.Orgs)
	}

	auth := cfg.Auths["sandbox-agent"]
	if auth.Type != AuthTypePlatform || auth.ClientID != "agent-cid" {
		t.Fatalf("auth = %+v", auth)
	}
	if auth.EffectiveScope() != PlatformScopeTenant || auth.EffectiveAccess() != AccessReadonly {
		t.Fatalf("auth scope/access = %s/%s", auth.EffectiveScope(), auth.EffectiveAccess())
	}
	if len(auth.Tenants) != 1 || auth.Tenants[0] != "sandbox" {
		t.Fatalf("auth contexts = %v", auth.Tenants)
	}
	if got := auth.normalizedChannels(); len(got) != 2 || got[0] != "cli" || got[1] != "mcp" {
		t.Fatalf("auth channels = %v", got)
	}
	if len(auth.DenyBuiltins) != 1 || auth.DenyBuiltins[0] != "computer_erase" {
		t.Fatalf("auth deny = %v", auth.DenyBuiltins)
	}
	if len(auth.AllowBuiltins) != 1 || auth.AllowBuiltins[0] != "policies" {
		t.Fatalf("auth allow = %v", auth.AllowBuiltins)
	}
}

func TestLoadConfig_AgentPolicyAndWorkplaceDenyApply(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentIdentityDoc)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("features:\n  use_agent: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.ClassicInstanceOnly() {
		t.Fatal("agent classic_instance_only should apply")
	}
	if cfg.DefaultTenant(ChannelCLI) != "sandbox" {
		t.Fatalf("cli default = %q", cfg.DefaultTenant(ChannelCLI))
	}
	if cfg.ChannelAccessFor(ChannelCLI) != AccessReadonly {
		t.Fatalf("cli access = %s", cfg.ChannelAccessFor(ChannelCLI))
	}
	if err := checkBuiltinLayers(cfg, ChannelCLI, "sandbox", "sandbox-agent", "computer_erase"); err == nil {
		t.Fatal("channel deny should block computer_erase")
	}
	if err := checkBuiltinLayers(cfg, ChannelCLI, "sandbox", "sandbox-agent", "uuid_new"); err == nil {
		t.Fatal("workplace deny * should block uuid_new")
	}
	if err := checkBuiltinLayers(cfg, ChannelCLI, "sandbox", "sandbox-agent", "policies"); err != nil {
		t.Fatalf("policies should be allowed: %v", err)
	}
	if !cfg.AuthFromAgent("sandbox-agent") || !cfg.ContextFromAgent("sandbox") {
		t.Fatal("agent ownership flags")
	}
}

func TestLoadConfig_UserChannelShadowsAgentPolicy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, agentIdentityDoc)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "features:\n  use_agent: true\nchannels:\n  cli:\n    access: full\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ChannelAccessFor(ChannelCLI) != AccessFull {
		t.Fatalf("user channel must win: %+v", cfg.Channels[ChannelCLI])
	}
	dups := AgentDuplicates(cfg)
	var channelDup *AgentDuplicate
	for i := range dups {
		if dups[i].Kind == "channel" && dups[i].Name == ChannelCLI {
			channelDup = &dups[i]
			break
		}
	}
	if channelDup == nil || !channelDup.Differs {
		t.Fatalf("expected a differing cli channel duplicate: %+v", dups)
	}
	if !strings.Contains(channelDup.Detail, "access") {
		t.Fatalf("channel detail = %q", channelDup.Detail)
	}
}

func TestLoadConfig_PolicyOnlyAgentIsInPlay(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, `{
	  "contexts": {"sandbox": {"url": "https://agent.example.com"}},
	  "features": {"classic_instance_only": true},
	  "channels": {"cli": {"access": "readonly"}}
	}`)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("features:\n  use_agent: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.HasAgentConfig() {
		t.Fatal("policy-only Agent documents must count as in play")
	}
	if !cfg.ChannelFromAgent(ChannelCLI) || !cfg.ClassicInstanceOnlyFromAgent() {
		t.Fatal("channel/classic ownership flags")
	}
	if !cfg.ClassicInstanceOnly() || cfg.ChannelAccessFor(ChannelCLI) != AccessReadonly {
		t.Fatal("policy should apply after LoadConfig")
	}
	sum := cfg.AgentConfigSummary()
	if !strings.Contains(sum, "cli") || !strings.Contains(sum, "classic_instance_only") {
		t.Fatalf("summary = %q", sum)
	}
}

func TestParseAgentConfigDocumentDropsUseAgent(t *testing.T) {
	cfg, err := parseAgentConfigDocument(json.RawMessage(`{"features":{"use_agent":true}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Features.UseAgent != nil {
		t.Fatal("stale features.use_agent must be stripped")
	}
	if cfg.Features.ClassicInstanceOnly != nil {
		t.Fatal("empty features after strip")
	}
}

func TestAgentPruneDropsChannelAndClassic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, `{
	  "context": {"default": "sandbox"},
	  "contexts": {"sandbox": {"url": "https://agent.example.com"}},
	  "features": {"classic_instance_only": true},
	  "channels": {"cli": {"access": "readonly", "deny_builtins": ["computer_erase"]}}
	}`)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "features:\n  use_agent: true\n  classic_instance_only: false\nchannels:\n  cli:\n    access: full\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	dups := AgentDuplicates(cfg)
	if len(dups) != 2 {
		t.Fatalf("duplicates = %+v", dups)
	}
	res, err := PruneAgentDuplicates(cfg, false)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(res.Channels) != 1 || res.Channels[0] != ChannelCLI {
		t.Fatalf("pruned channels = %v", res.Channels)
	}
	if len(res.Features) != 1 || res.Features[0] != "classic_instance_only" {
		t.Fatalf("pruned features = %v", res.Features)
	}
	if cfg.ChannelAccessFor(ChannelCLI) != AccessReadonly {
		t.Fatalf("agent channel should win now: %+v", cfg.Channels[ChannelCLI])
	}
	if !cfg.ClassicInstanceOnly() {
		t.Fatal("agent classic_instance_only should win now")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	if strings.Contains(saved, "classic_instance_only") || strings.Contains(saved, "channels:") {
		t.Fatalf("policy duplicate survived:\n%s", saved)
	}
	if !strings.Contains(saved, "use_agent: true") {
		t.Fatalf("use_agent lost:\n%s", saved)
	}
}

func TestAgentPruneDropsLocalChannelPinsWhenAgentOmitsChannels(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	serveAgentConfig(t, `{
	  "context": {"default": "prod"},
	  "contexts": {"prod": {"url": "https://prod.example.com"}, "sandbox": {"url": "https://sandbox.example.com"}}
	}`)

	path := filepath.Join(dir, "buddy", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	user := "features:\n  use_agent: true\nchannels:\n  cli:\n    context: sandbox\n  ide:\n    context: prod\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DefaultTenant(ChannelCLI) != "sandbox" {
		t.Fatalf("cli still uses the local pin: %q", cfg.DefaultTenant(ChannelCLI))
	}
	dups := AgentDuplicates(cfg)
	if len(dups) != 2 {
		t.Fatalf("duplicates = %+v", dups)
	}
	res, err := PruneAgentDuplicates(cfg, false)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(res.Channels) != 2 {
		t.Fatalf("pruned channels = %v", res.Channels)
	}
	if cfg.DefaultTenant(ChannelCLI) != "prod" {
		t.Fatalf("cli should fall through to the Agent default: %q", cfg.DefaultTenant(ChannelCLI))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	if strings.Contains(saved, "channels:") {
		t.Fatalf("channels: should have been handed to the Agent:\n%s", saved)
	}
	if !strings.Contains(saved, "use_agent: true") {
		t.Fatalf("use_agent lost:\n%s", saved)
	}
}

func TestWorkplaceDifferenceIncludesDenyAllow(t *testing.T) {
	user := TenantConfig{URL: "https://a.example.com", DenyBuiltins: []string{"*"}}
	agent := TenantConfig{URL: "https://a.example.com", DenyBuiltins: []string{"computer_erase"}}
	detail := workplaceDifference(user, agent)
	if !strings.Contains(detail, "deny_builtins") {
		t.Fatalf("detail = %q", detail)
	}
}
