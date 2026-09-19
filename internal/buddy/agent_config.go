package buddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"buddy/internal/buddy/agent"

	"gopkg.in/yaml.v3"
)

// agentConfigFetch is the Agent's config op, swapped out in tests.
var agentConfigFetch = agent.Config

// agentConfigJSON asks the running Buddy Agent for its config document. The
// Agent serves this while locked: workplaces and auths are identity, not
// secrets, so resolving a session never needs a biometric prompt.
func agentConfigJSON() (json.RawMessage, error) {
	if !agent.Enabled() {
		return nil, agentDisabledError()
	}
	raw, err := agentConfigFetch(context.Background())
	if err != nil {
		if errors.Is(err, agent.ErrAbsent) || errors.Is(err, agent.ErrUnsupported) {
			return nil, agentAbsentError(err)
		}
		return nil, fmt.Errorf("buddy agent config: %w", err)
	}
	return raw, nil
}

func agentDisabledError() error {
	return fmt.Errorf("features.use_agent/force_agent is on but the Buddy Agent is disabled (%s); unset it, or turn off features.use_agent / features.force_agent", agent.EnvDisable)
}

func agentAbsentError(err error) error {
	return fmt.Errorf("features.use_agent/force_agent is on but the Buddy Agent is not running (%v); start Buddy Agent, or turn off features.use_agent / features.force_agent", err)
}

// layersForceAgent is true when env, MDM, or user YAML requires the Agent
// and ignores config.yaml. MDM force_agent: true is sticky (user cannot
// turn it off).
func layersForceAgent(mdm, user *Config) bool {
	if v, ok := parseBoolishEnv(EnvForceAgent); ok {
		return v
	}
	if mdm != nil && mdm.Features.ForceAgent != nil && *mdm.Features.ForceAgent {
		return true
	}
	return user != nil && user.Features.ForceAgent != nil && *user.Features.ForceAgent
}

func agentNeedsConfigError() error {
	return fmt.Errorf("Buddy Agent needs config; add a workplace in Buddy Agent")
}

// IsAgentNeedsConfig reports the empty-Agent load error (no workplaces in
// Agent, user, or MDM). buddy agent enable/disable/force still run so the
// flag can be written.
func IsAgentNeedsConfig(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Buddy Agent needs config")
}

// SetUseAgent writes features.use_agent to config.yaml (true or false).
// Agent workplaces are not copied into the user file.
func SetUseAgent(cfg *Config, on bool) error {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.Features.UseAgent = boolPtr(on)
	if cfg.user == nil {
		cfg.user = &Config{}
	}
	cfg.user.Features.UseAgent = boolPtr(on)
	if cfg.agent == nil && cfg.mdm == nil {
		// Force the user-overlay save path so we do not invent context.default.
		cfg.mdm = &Config{}
	}
	return SaveConfig(cfg)
}

// SetForceAgent writes features.force_agent to config.yaml (true or false).
// Agent workplaces are not copied into the user file.
func SetForceAgent(cfg *Config, on bool) error {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.Features.ForceAgent = boolPtr(on)
	if cfg.user == nil {
		cfg.user = &Config{}
	}
	cfg.user.Features.ForceAgent = boolPtr(on)
	if cfg.agent == nil && cfg.mdm == nil {
		cfg.mdm = &Config{}
	}
	return SaveConfig(cfg)
}

// loadAgentConfig converts the Agent's JSON document into a Config layer. The
// JSON mirrors config.yaml keys, so it goes through the same parser as the user
// file: dual-read, org folding, and validation are shared rather than
// reimplemented.
func loadAgentConfig() (*Config, error) {
	raw, err := agentConfigJSON()
	if err != nil {
		return nil, err
	}
	cfg, err := parseAgentConfigDocument(raw)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// tryLoadAgentConfig is the first-run probe: missing or unsupported Agent is
// not fatal (setup still runs). A contactable Agent is used as the Agent
// layer without writing features.use_agent. Other socket/document errors fail.
func tryLoadAgentConfig() (*Config, error) {
	if !agent.Enabled() {
		return nil, nil
	}
	raw, err := agentConfigFetch(context.Background())
	if err != nil {
		if errors.Is(err, agent.ErrAbsent) || errors.Is(err, agent.ErrUnsupported) {
			return nil, nil
		}
		return nil, fmt.Errorf("buddy agent config: %w", err)
	}
	return parseAgentConfigDocument(raw)
}

func configLooksUnconfigured(cfg *Config) bool {
	if cfg == nil {
		return true
	}
	return len(cfg.Tenants) == 0 && len(cfg.Orgs) == 0 && len(cfg.Auths) == 0
}

func agentHasWorkplaces(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	return len(cfg.Tenants) > 0 || len(cfg.Orgs) > 0
}

// shouldDiscoverAgent is true when config.yaml (and MDM) have no workplaces
// yet, the Agent is available on this platform, and the user has not turned
// use_agent off. A contactable Agent is then used in memory only. Linux/Windows
// stay on the existing first-run path unless BUDDY_AGENT_SOCK is set.
func shouldDiscoverAgent(user, mdm *Config) bool {
	if !agent.Enabled() {
		return false
	}
	if v, ok := parseBoolishEnv(EnvUseAgent); ok && !v {
		return false
	}
	if user != nil && user.Features.UseAgent != nil && !*user.Features.UseAgent {
		return false
	}
	if mdm != nil && mdm.Features.UseAgent != nil && !*mdm.Features.UseAgent {
		return false
	}
	return configLooksUnconfigured(user) && configLooksUnconfigured(mdm)
}

func parseAgentConfigDocument(raw json.RawMessage) (*Config, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("buddy agent config: malformed document: %w", err)
	}
	if len(doc) == 0 {
		return &Config{}, nil
	}
	if feat, ok := doc["features"].(map[string]any); ok {
		delete(feat, "use_agent")
		delete(feat, "force_agent")
		if len(feat) == 0 {
			delete(doc, "features")
		}
	}
	for _, key := range []string{"auths"} {
		entries, ok := doc[key].(map[string]any)
		if !ok {
			continue
		}
		for name, entry := range entries {
			row, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			// The Agent must never send secret material; drop it if it ever does.
			for _, secretKey := range []string{"password", "client_secret", "clientSecret"} {
				if _, found := row[secretKey]; found {
					delete(row, secretKey)
				}
			}
			entries[name] = row
		}
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("buddy agent config: %w", err)
	}
	cfg, _, err := parseAndMigrateUserYAML(data)
	if err != nil {
		return nil, fmt.Errorf("buddy agent config: %w", err)
	}
	return cfg, nil
}

// HasAgentConfig reports whether this config was merged with a Buddy Agent
// layer (workplaces, auths, channels, or classic_instance_only).
func (cfg *Config) HasAgentConfig() bool {
	if cfg == nil || cfg.agent == nil {
		return false
	}
	a := cfg.agent
	return len(a.Tenants) > 0 || len(a.Orgs) > 0 || len(a.Auths) > 0 ||
		len(a.Channels) > 0 || a.Features.ClassicInstanceOnly != nil
}

// ContextFromAgent reports whether workplace name comes from the Agent.
func (cfg *Config) ContextFromAgent(name string) bool {
	if cfg == nil || cfg.agent == nil {
		return false
	}
	_, ok := cfg.agent.Tenants[name]
	return ok
}

// AuthFromAgent reports whether auth name comes from the Agent. buddy never
// resolves a vault secret for those: the Agent mints their tokens.
func (cfg *Config) AuthFromAgent(name string) bool {
	if cfg == nil || cfg.agent == nil {
		return false
	}
	_, ok := cfg.agent.Auths[name]
	return ok
}

// ChannelFromAgent reports whether channels.<name> comes from the Agent.
func (cfg *Config) ChannelFromAgent(name string) bool {
	if cfg == nil || cfg.agent == nil || cfg.agent.Channels == nil {
		return false
	}
	_, ok := cfg.agent.Channels[normalizeBuddyChannel(name)]
	return ok
}

// ClassicInstanceOnlyFromAgent reports whether features.classic_instance_only
// comes from the Agent document.
func (cfg *Config) ClassicInstanceOnlyFromAgent() bool {
	return cfg != nil && cfg.agent != nil && cfg.agent.Features.ClassicInstanceOnly != nil
}

// AgentConfigSummary is a short line for `buddy cred status` (empty if none).
func (cfg *Config) AgentConfigSummary() string {
	if !cfg.HasAgentConfig() {
		return ""
	}
	var parts []string
	if names := sortedMapKeys(cfg.agent.Tenants); len(names) > 0 {
		parts = append(parts, "contexts "+strings.Join(names, ", "))
	}
	if names := sortedMapKeys(cfg.agent.Auths); len(names) > 0 {
		parts = append(parts, "auths "+strings.Join(names, ", "))
	}
	if names := sortedMapKeys(cfg.agent.Channels); len(names) > 0 {
		parts = append(parts, "channels "+strings.Join(names, ", "))
	}
	if cfg.agent.Features.ClassicInstanceOnly != nil {
		parts = append(parts, "classic_instance_only")
	}
	if len(parts) == 0 {
		return ""
	}
	return "agent: " + strings.Join(parts, "; ")
}

// AgentDuplicate is one config.yaml entry the Buddy Agent also provides. The
// user file wins per name, so a duplicate silently shadows the Agent's copy.
type AgentDuplicate struct {
	Kind    string // context | auth | channel | feature
	Name    string
	Differs bool   // the local copy is not identical to the Agent's
	Detail  string // what differs (empty when identical)
}

// AgentDuplicates lists config.yaml entries that the Agent also serves,
// sorted contexts, auths, channels, then features. Empty when there is no
// Agent layer.
func AgentDuplicates(cfg *Config) []AgentDuplicate {
	if cfg == nil || cfg.agent == nil || cfg.user == nil {
		return nil
	}
	var out []AgentDuplicate
	shared := map[string]bool{}
	for name := range cfg.user.Tenants {
		if _, ok := cfg.agent.Tenants[name]; ok {
			shared[name] = true
		}
	}
	for _, name := range sortedMapKeys(cfg.user.Tenants) {
		if !shared[name] || !workplaceChildrenAllShared(cfg.user, name, shared) {
			continue
		}
		detail := workplaceDifference(cfg.user.Tenants[name], cfg.agent.Tenants[name])
		out = append(out, AgentDuplicate{Kind: "context", Name: name, Differs: detail != "", Detail: detail})
	}
	for _, name := range sortedMapKeys(cfg.user.Auths) {
		agentA, ok := cfg.agent.Auths[name]
		if !ok {
			continue
		}
		userA := cfg.user.Auths[name]
		detail := ""
		if !authEntryEqual(userA, agentA) {
			detail = strings.TrimSuffix(strings.TrimSpace(
				describeDifference("type", userA.Type, agentA.Type)+
					describeDifference("clientId", userA.ClientID, agentA.ClientID)+
					describeDifference("username", userA.Username, agentA.Username)+
					describeDifference("scope", userA.EffectiveScope(), agentA.EffectiveScope())+
					describeDifference("access", userA.EffectiveAccess(), agentA.EffectiveAccess())+
					describeDifference("contexts", strings.Join(userA.Tenants, ","), strings.Join(agentA.Tenants, ","))+
					describeDifference("channels", strings.Join(userA.normalizedChannels(), ","), strings.Join(agentA.normalizedChannels(), ","))+
					describeDifference("deny_builtins", strings.Join(userA.DenyBuiltins, ","), strings.Join(agentA.DenyBuiltins, ","))+
					describeDifference("allow_builtins", strings.Join(userA.AllowBuiltins, ","), strings.Join(agentA.AllowBuiltins, ",")),
			), ";")
		}
		out = append(out, AgentDuplicate{Kind: "auth", Name: name, Differs: detail != "", Detail: detail})
	}
	// Channel policy is an Agent surface (Policy tab), including an omitted
	// channel — a local cli/mcp/ide pin from `cred use` still overlays the
	// Agent's context.default.
	for _, name := range sortedMapKeys(cfg.user.Channels) {
		if !isBuddyChannel(name) {
			continue
		}
		userP := cfg.user.Channels[name]
		var agentP ChannelPolicy
		if cfg.agent.Channels != nil {
			agentP = cfg.agent.Channels[name]
		}
		detail := channelPolicyDifference(userP, agentP)
		out = append(out, AgentDuplicate{Kind: "channel", Name: name, Differs: detail != "", Detail: detail})
	}
	if cfg.user.Features.ClassicInstanceOnly != nil && cfg.agent.Features.ClassicInstanceOnly != nil {
		detail := describeDifference(
			"classic_instance_only",
			formatBoolPtr(cfg.user.Features.ClassicInstanceOnly),
			formatBoolPtr(cfg.agent.Features.ClassicInstanceOnly),
		)
		detail = strings.TrimSuffix(strings.TrimSpace(detail), ";")
		out = append(out, AgentDuplicate{
			Kind:    "feature",
			Name:    "classic_instance_only",
			Differs: detail != "",
			Detail:  detail,
		})
	}
	return out
}

// workplaceChildrenAllShared reports whether every workplace hanging off this
// one in config.yaml is also served by the Agent. A parent whose children are
// not all shared has to stay: removing it would orphan a local-only workplace.
func workplaceChildrenAllShared(user *Config, name string, shared map[string]bool) bool {
	if user == nil || len(user.Orgs) == 0 {
		return true
	}
	loc, ok := locateInOrgs(user.Orgs, name)
	if !ok {
		return true
	}
	for _, child := range loc.directChildren(user.Orgs) {
		if !shared[child] {
			return false
		}
	}
	return true
}

func channelPolicyDifference(user, agent ChannelPolicy) string {
	if channelPolicyEqual(user, agent) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(
		describeDifference("access", user.EffectiveAccess(), agent.EffectiveAccess())+
			describeDifference("context", user.Tenant, agent.Tenant)+
			describeDifference("contexts", strings.Join(user.Tenants, ","), strings.Join(agent.Tenants, ","))+
			describeDifference("deny_builtins", strings.Join(user.DenyBuiltins, ","), strings.Join(agent.DenyBuiltins, ","))+
			describeDifference("allow_builtins", strings.Join(user.AllowBuiltins, ","), strings.Join(agent.AllowBuiltins, ",")),
	), ";")
}

func workplaceDifference(user, agent TenantConfig) string {
	if tenantConfigEqual(user, agent) {
		return ""
	}
	detail := describeDifference("url", user.URL, agent.URL) +
		describeDifference("region", user.Region, agent.Region) +
		describeDifference("environmentId", user.EnvironmentID, agent.EnvironmentID) +
		describeDifference("tenantId", user.TenantID, agent.TenantID) +
		describeDifference("deny_builtins", strings.Join(user.DenyBuiltins, ","), strings.Join(agent.DenyBuiltins, ",")) +
		describeDifference("allow_builtins", strings.Join(user.AllowBuiltins, ","), strings.Join(agent.AllowBuiltins, ","))
	return strings.TrimSuffix(strings.TrimSpace(detail), ";")
}

func describeDifference(field, user, agent string) string {
	if strings.TrimSpace(user) == strings.TrimSpace(agent) {
		return ""
	}
	return fmt.Sprintf("%s %s → %s; ", field, quoteOrNone(user), quoteOrNone(agent))
}

func formatBoolPtr(p *bool) string {
	if p == nil {
		return ""
	}
	if *p {
		return "true"
	}
	return "false"
}

func quoteOrNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none)"
	}
	return fmt.Sprintf("%q", s)
}

// AgentPruneResult reports what PruneAgentDuplicates removed.
type AgentPruneResult struct {
	Contexts []string
	Auths    []string
	Channels []string
	Features []string
	Secrets  []string // vault secrets deleted (dropSecrets only)
}

// PruneAgentDuplicates removes the duplicated entries from config.yaml so the
// Agent's copy is the only one, and rewrites the file. cfg is replaced with the
// re-merged result. With dropSecrets, the now-unused vault secrets for those
// auths are deleted too — the Agent holds its own.
func PruneAgentDuplicates(cfg *Config, dropSecrets bool) (AgentPruneResult, error) {
	var res AgentPruneResult
	if cfg == nil {
		return res, fmt.Errorf("internal error: config is nil")
	}
	dups := AgentDuplicates(cfg)
	if len(dups) == 0 {
		return res, nil
	}
	pruned := cloneConfig(cfg.user)
	if pruned == nil {
		pruned = &Config{}
	}
	var contexts []string
	for _, d := range dups {
		switch d.Kind {
		case "auth":
			delete(pruned.Auths, d.Name)
			res.Auths = append(res.Auths, d.Name)
		case "context":
			contexts = append(contexts, d.Name)
		case "channel":
			delete(pruned.Channels, d.Name)
			res.Channels = append(res.Channels, d.Name)
		case "feature":
			if d.Name == "classic_instance_only" {
				pruned.Features.ClassicInstanceOnly = nil
				res.Features = append(res.Features, d.Name)
			}
		}
	}
	// A parent only leaves the orgs tree once its children have, so keep
	// sweeping while the previous pass removed something.
	for len(contexts) > 0 {
		var blocked []string
		for _, name := range contexts {
			if err := removeWorkplaceFromOrgs(pruned, name); err != nil {
				blocked = append(blocked, name)
				continue
			}
			delete(pruned.Tenants, name)
			res.Contexts = append(res.Contexts, name)
		}
		if len(blocked) == len(contexts) {
			break
		}
		contexts = blocked
	}
	sort.Strings(res.Contexts)
	// The Agent supplies the default too; keep the user's only when it differs.
	if cfg.agent != nil && strings.TrimSpace(pruned.Tenant.Default) == strings.TrimSpace(cfg.agent.Tenant.Default) {
		pruned.Tenant.Default = ""
	}

	merged := overlayConfig(overlayConfig(cfg.agent, cfg.mdm), pruned)
	if merged == nil {
		merged = &Config{}
	}
	merged.user = cloneConfig(pruned)
	merged.mdm = cloneConfig(cfg.mdm)
	merged.agent = cloneConfig(cfg.agent)
	if err := expandOrgs(merged); err != nil {
		return res, fmt.Errorf("pruned config: %w", err)
	}
	if err := merged.ValidateTenantSelectionAndChannels(); err != nil {
		return res, fmt.Errorf("pruned config: %w", err)
	}
	if err := SaveConfig(merged); err != nil {
		return res, err
	}
	*cfg = *merged

	if dropSecrets && len(res.Auths) > 0 {
		deleted, err := deleteVaultSecretsFor(res.Auths)
		if err != nil {
			return res, err
		}
		res.Secrets = deleted
	}
	return res, nil
}

// deleteVaultSecretsFor removes the local copies of secrets the Agent now owns.
// Only names that actually have a vault entry are touched (and reported).
func deleteVaultSecretsFor(names []string) ([]string, error) {
	entries, err := ListCredentialSecrets()
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}
	var deleted []string
	seen := map[string]bool{}
	for _, e := range entries {
		if !wanted[e.AuthName] || seen[e.AuthName] {
			continue
		}
		if err := DeleteCredentialSecrets(e.AuthName, true, true); err != nil {
			return deleted, fmt.Errorf("delete secrets for %q: %w", e.AuthName, err)
		}
		seen[e.AuthName] = true
		deleted = append(deleted, e.AuthName)
	}
	return deleted, nil
}

// errAgentOwned explains why a config edit was refused.
func errAgentOwned(kind, name string) error {
	return fmt.Errorf("%s %q is owned by the Buddy Agent; edit it in the Buddy Agent window (config.yaml only overlays it)", kind, name)
}
