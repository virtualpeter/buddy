package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"buddy/internal/buddy"
	"buddy/internal/buddy/agent"
	"buddy/internal/clispec"

	"gopkg.in/yaml.v3"
)

// runAgent is the 'buddy agent' group: read-only views of the optional macOS
// Buddy Agent. Nothing here mints, pushes a secret, or unlocks — unlock is a
// tray action, and the Agent owns its own credentials.
func runAgent(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	subs := []string{clispec.SubStatus, "config", "integrations", "tokens", "lock", "prune", clispec.SubEnable, clispec.SubDisable, clispec.SubForce, clispec.SubHelp}
	if err := resolveArg0(clispec.CmdAgent, args, subs); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == clispec.SubHelp {
		fmt.Print(helpAgent)
		return nil
	}
	switch args[0] {
	case clispec.SubStatus:
		return runAgentStatus(args[1:])
	case "config":
		return runAgentConfig(args[1:])
	case "integrations":
		return runAgentIntegrations(args[1:])
	case "tokens":
		return runAgentTokens(args[1:])
	case "lock":
		return runAgentLock(args[1:])
	case "prune":
		return runAgentPrune(cfg, opts, args[1:])
	case clispec.SubEnable:
		return runAgentSetUseAgent(cfg, true, args[1:])
	case clispec.SubDisable:
		return runAgentSetUseAgent(cfg, false, args[1:])
	case clispec.SubForce:
		return runAgentSetForceAgent(cfg, args[1:])
	default:
		return errUnknownSubcommand(clispec.CmdAgent, args[0], subs)
	}
}

// runAgentPrune drops config.yaml entries the Agent already serves, so an
// imported setup stops shadowing the Agent's copy with a local duplicate.
func runAgentPrune(cfg *buddy.Config, opts buddy.Opts, args []string) error {
	dryRun, dropSecrets := false, false
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
		case "--secrets":
			dropSecrets = true
		case "-y":
			opts.SkipConfirm = true
		case clispec.SubHelp:
			fmt.Print(helpAgent)
			return nil
		default:
			return fmt.Errorf("unknown argument %q; usage: buddy agent prune [--dry-run] [--secrets] [-y]", a)
		}
	}
	if !cfg.HasAgentConfig() {
		return fmt.Errorf("no Buddy Agent config is in play; start Buddy Agent (empty config.yaml uses it automatically), or run 'buddy agent enable' (or %s=1)", buddy.EnvUseAgent)
	}
	dups := buddy.AgentDuplicates(cfg)
	if len(dups) == 0 {
		fmt.Println("config.yaml has no entries the Buddy Agent also provides.")
		return nil
	}

	fmt.Printf("These %d entr(ies) exist in both config.yaml and the Buddy Agent:\n\n", len(dups))
	differs := 0
	for _, d := range dups {
		note := "same as the Agent"
		if d.Differs {
			differs++
			note = "local differs: " + d.Detail
		}
		fmt.Printf("  %-8s %-24s %s\n", d.Kind, d.Name, note)
	}
	fmt.Println("\nThe local copy wins today, so removing it hands the entry to the Agent.")
	if differs > 0 {
		fmt.Printf("%d of them differ: the Agent's value takes over.\n", differs)
	}
	if dropSecrets {
		fmt.Println("--secrets: their vault secrets are deleted too (the Agent keeps its own).")
	}
	if dryRun {
		fmt.Println("\n--dry-run: nothing changed.")
		return nil
	}
	if !opts.SkipConfirm && !buddy.AskYesNo("\nRemove them from config.yaml?", false, nil) {
		return nil
	}

	res, err := buddy.PruneAgentDuplicates(cfg, dropSecrets)
	if err != nil {
		return err
	}
	fmt.Printf("Removed %d context(s) and %d auth(s) from config.yaml.\n", len(res.Contexts), len(res.Auths))
	if len(res.Channels) > 0 {
		fmt.Printf("Removed %d channel(s): %s\n", len(res.Channels), strings.Join(res.Channels, ", "))
	}
	if len(res.Features) > 0 {
		fmt.Printf("Removed %d feature(s): %s\n", len(res.Features), strings.Join(res.Features, ", "))
	}
	if len(res.Secrets) > 0 {
		fmt.Printf("Deleted %d vault secret(s): %s\n", len(res.Secrets), strings.Join(res.Secrets, ", "))
	}
	fmt.Println("Run 'buddy cred status' to see the Agent as the source (SECRET=agent).")
	return nil
}

// runAgentSetUseAgent writes features.use_agent to config.yaml.
func runAgentSetUseAgent(cfg *buddy.Config, on bool, args []string) error {
	sub := clispec.SubDisable
	if on {
		sub = clispec.SubEnable
	}
	if showHelp, err := agentNoArgs(sub, args); err != nil {
		return err
	} else if showHelp {
		fmt.Print(helpAgent)
		return nil
	}
	if cfg == nil {
		cfg = &buddy.Config{}
	}
	if err := buddy.SetUseAgent(cfg, on); err != nil {
		return err
	}
	fmt.Printf("Wrote features.use_agent: %v\n", on)
	if v, ok := parseBoolishEnv(buddy.EnvUseAgent); ok && v != on {
		fmt.Fprintf(os.Stderr, "warning: %s=%s overrides config.yaml for this process\n", buddy.EnvUseAgent, os.Getenv(buddy.EnvUseAgent))
	}
	return nil
}

// runAgentSetForceAgent writes features.force_agent to config.yaml.
func runAgentSetForceAgent(cfg *buddy.Config, args []string) error {
	on := true
	for _, a := range args {
		switch a {
		case "--off":
			on = false
		case clispec.SubHelp:
			fmt.Print(helpAgent)
			return nil
		default:
			return fmt.Errorf("unknown argument %q; usage: buddy agent force [--off]", a)
		}
	}
	if cfg == nil {
		cfg = &buddy.Config{}
	}
	if err := buddy.SetForceAgent(cfg, on); err != nil {
		return err
	}
	fmt.Printf("Wrote features.force_agent: %v\n", on)
	if v, ok := parseBoolishEnv(buddy.EnvForceAgent); ok && v != on {
		fmt.Fprintf(os.Stderr, "warning: %s=%s overrides config.yaml for this process\n", buddy.EnvForceAgent, os.Getenv(buddy.EnvForceAgent))
	}
	if !on && cfg.ManagedForceAgent() {
		fmt.Fprintln(os.Stderr, "warning: managed preferences still have force_agent: true; that wins over this file")
	}
	return nil
}

func parseBoolishEnv(key string) (value, ok bool) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return false, false
	}
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// isAgentFlagWrite is true for `buddy agent enable|disable|force` (unique prefix).
func isAgentEnableDisable(args []string) bool {
	if len(args) < 2 {
		return false
	}
	id, _, ok := clispec.ResolveCommand(args[0])
	if !ok || id != clispec.CmdAgent {
		return false
	}
	sub := []string{args[1]}
	if err := resolveArg0(clispec.CmdAgent, sub, []string{clispec.SubEnable, clispec.SubDisable, clispec.SubForce}); err != nil {
		return false
	}
	return sub[0] == clispec.SubEnable || sub[0] == clispec.SubDisable || sub[0] == clispec.SubForce
}

// agentNoArgs rejects stray arguments and prints help on request.
func agentNoArgs(sub string, args []string) (help bool, err error) {
	for _, a := range args {
		if a == clispec.SubHelp {
			return true, nil
		}
		return false, fmt.Errorf("unknown argument %q; usage: buddy agent %s", a, sub)
	}
	return false, nil
}

// runAgentStatus reports whether the Agent is listening. It never mints: it is
// the smoke test for the socket and the lock state.
func runAgentStatus(args []string) error {
	if showHelp, err := agentNoArgs(clispec.SubStatus, args); err != nil {
		return err
	} else if showHelp {
		fmt.Print(helpAgent)
		return nil
	}
	path, perr := agent.SocketPath()
	if path != "" {
		fmt.Printf("socket:       %s\n", path)
	}
	if !agent.Enabled() {
		fmt.Printf("agent:        disabled (%s, or not macOS)\n", agent.EnvDisable)
		return nil
	}
	if perr != nil {
		return perr
	}
	info, err := agent.Status(context.Background())
	switch {
	case errors.Is(err, agent.ErrAbsent):
		fmt.Println("present:      no (buddy uses the local vault)")
		return nil
	case err != nil:
		return err
	}
	fmt.Println("present:      yes")
	fmt.Printf("unlocked:     %v\n", info.Unlocked)
	if info.Version != "" {
		fmt.Printf("version:      %s\n", info.Version)
	}
	if info.PeerCheck != "" {
		fmt.Printf("peer check:   %s\n", info.PeerCheck)
	}
	fmt.Printf("integrations: %d\n", info.Integrations)
	fmt.Printf("tokens:       %d\n", info.Tokens)
	if info.Presence != "" {
		fmt.Printf("presence:     %s\n", describePresence(info))
	}
	fmt.Printf("idle lock:    %s\n", describeIdleLock(info.IdleLock))
	if info.ConfigIntegrity != "" {
		fmt.Printf("integrity:    %s\n", describeConfigIntegrity(info.ConfigIntegrity))
	}
	if info.ManagedPrefs != "" {
		fmt.Printf("managed prefs: %s\n", info.ManagedPrefs)
	}
	if !info.Unlocked {
		fmt.Println("\nConfig is served while locked. " + describeLockedHint(info))
	}
	return nil
}

func describeConfigIntegrity(state string) string {
	switch state {
	case "ok":
		return "ok (matches Keychain seal)"
	case "sealed":
		return "sealed (verified when minting)"
	case "missing":
		return "missing (no Keychain seal yet)"
	case "tampered":
		return "tampered (mint will fail until the Agent re-seals)"
	case "session_stale":
		return "session stale (Keychain no longer accepts the unlock; the Agent re-prompts on the next mint)"
	case "unavailable":
		return "unavailable (unsigned Agent has no seal)"
	}
	return state
}

func describePresence(info agent.Info) string {
	switch info.Presence {
	case "acl":
		// Signed Agent: the prompt releases the keychain item itself.
		return "Keychain userPresence" + describePromptWait(info)
	case "simulated":
		// Unsigned Agent: LocalAuthentication answers yes or no and the Agent
		// chooses to honour it. The secret is not behind the prompt.
		return "LocalAuthentication, unsigned agent" + describePromptWait(info)
	case "unavailable":
		return "unavailable (menu bar only)"
	}
	return info.Presence
}

func describePromptWait(info agent.Info) string {
	if info.UnlockTimeout == 0 {
		return " (prompting disabled)"
	}
	return fmt.Sprintf(" (prompt waits %s)", info.UnlockTimeout)
}

func describeIdleLock(d time.Duration) string {
	if d <= 0 {
		return "never"
	}
	return d.String()
}

func describeLockedHint(info agent.Info) string {
	if agent.PromptSuppressed() {
		return fmt.Sprintf(
			"%s=1 is set, so unlock from the Buddy Agent menu bar item before minting.",
			agent.EnvNoPrompt)
	}
	switch info.Presence {
	case "unavailable":
		return "This Mac cannot show a presence prompt; unlock from the Buddy Agent menu bar item."
	case "":
		// Pre-0.3 Agent: it will not prompt.
		return "Unlock from the Buddy Agent menu bar item before minting."
	}
	if info.UnlockTimeout == 0 {
		return "Prompting is switched off; unlock from the Buddy Agent menu bar item before minting."
	}
	return "buddy will ask the Agent to unlock when it next needs a token."
}

// runAgentConfig prints the config document the Agent serves. It is identity
// only (no secrets), and the Agent serves it without an unlock.
func runAgentConfig(args []string) error {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case clispec.SubHelp:
			fmt.Print(helpAgent)
			return nil
		default:
			return fmt.Errorf("unknown argument %q; usage: buddy agent config [--json]", a)
		}
	}
	raw, err := agent.Config(context.Background())
	if err != nil {
		return err
	}
	if asJSON {
		var pretty any
		if err := json.Unmarshal(raw, &pretty); err != nil {
			return err
		}
		out, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc) == 0 {
		fmt.Println("The Buddy Agent has no integrations yet; add one in its window.")
		return nil
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	return nil
}

func runAgentIntegrations(args []string) error {
	if showHelp, err := agentNoArgs("integrations", args); err != nil {
		return err
	} else if showHelp {
		fmt.Print(helpAgent)
		return nil
	}
	rows, err := agent.Integrations(context.Background())
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("The Buddy Agent has no integrations yet; add one in its window.")
		return nil
	}
	fmt.Printf("%-16s  %-20s  %-9s  %-8s  %s\n", "CONTEXT", "AUTH", "TYPE", "SECRET", "NOTES")
	fmt.Printf("%-16s  %-20s  %-9s  %-8s  %s\n",
		"----------------", "--------------------", "---------", "--------", "-----")
	for _, row := range rows {
		note := row.Problem
		if note == "" {
			var bits []string
			if row.Scope != "" {
				bits = append(bits, row.Scope)
			}
			if row.URL != "" {
				bits = append(bits, row.URL)
			}
			note = strings.Join(bits, "  ")
		}
		fmt.Printf("%-16s  %-20s  %-9s  %-8s  %s\n", row.Context, row.Auth, row.Type, row.Secret, note)
	}
	fmt.Println("\nEdit these in the Buddy Agent window; 'buddy cred set' does not own them.")
	return nil
}

func runAgentTokens(args []string) error {
	if showHelp, err := agentNoArgs("tokens", args); err != nil {
		return err
	} else if showHelp {
		fmt.Print(helpAgent)
		return nil
	}
	rows, err := agent.Tokens(context.Background())
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("The Buddy Agent has no cached bearers.")
		return nil
	}
	fmt.Printf("%-20s  %-40s  %s\n", "AUTH", "BASE URL", "EXPIRES")
	for _, row := range rows {
		fmt.Printf("%-20s  %-40s  %s\n", row.Auth, row.BaseURL, row.ExpiresAt)
	}
	return nil
}

func runAgentLock(args []string) error {
	if showHelp, err := agentNoArgs("lock", args); err != nil {
		return err
	} else if showHelp {
		fmt.Print(helpAgent)
		return nil
	}
	info, err := agent.Lock(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("Buddy Agent locked (unlocked=%v); cached bearers dropped.\n", info.Unlocked)
	return nil
}

// writeSetupExportJSON writes the secrets-free export as JSON for the Buddy
// Agent's import picker (same content as the YAML export).
func writeSetupExportJSON(yamlData []byte, path string) error {
	var doc map[string]any
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return fmt.Errorf("convert export to JSON: %w", err)
	}
	out, err := json.MarshalIndent(jsonSafe(doc), "", "  ")
	if err != nil {
		return fmt.Errorf("convert export to JSON: %w", err)
	}
	out = append(out, '\n')
	if strings.TrimSpace(path) == "" || path == "-" {
		_, err = os.Stdout.Write(out)
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// jsonSafe converts YAML's map[any]any (nested mappings) into JSON-encodable maps.
func jsonSafe(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = jsonSafe(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[fmt.Sprint(k)] = jsonSafe(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = jsonSafe(item)
		}
		return out
	default:
		return value
	}
}
