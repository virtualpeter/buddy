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
