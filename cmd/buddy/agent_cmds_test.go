package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/buddy/agent"
	"buddy/internal/clispec"
)

func TestAgentCommandIsKnown(t *testing.T) {
	if !isKnownCommand(clispec.CmdAgent) {
		t.Fatal("agent should be a top-level command")
	}
	if id, _, ok := clispec.ResolveCommand("agen"); !ok || id != clispec.CmdAgent {
		t.Fatalf("prefix resolve = %q, %v", id, ok)
	}
}

func TestAgentUnknownSubcommand(t *testing.T) {
	t.Setenv(agent.EnvDisable, "1")
	cfg := &buddy.Config{}
	err := runAgent(cfg, buddy.Opts{}, []string{"push"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("err = %v, want an unknown-subcommand error", err)
	}
	// There is no unlock over the socket: unlock is a tray action.
	if err := runAgent(cfg, buddy.Opts{}, []string{"unlock"}); err == nil {
		t.Fatal("agent unlock must not be a subcommand")
	}
}

func TestAgentPruneNeedsAgentConfig(t *testing.T) {
	t.Setenv(agent.EnvDisable, "1")
	err := runAgentPrune(&buddy.Config{}, buddy.Opts{}, nil)
	if err == nil || !strings.Contains(err.Error(), "agent enable") {
		t.Fatalf("err = %v, want an enable hint", err)
	}
	if err := runAgentPrune(&buddy.Config{}, buddy.Opts{}, []string{"--nope"}); err == nil {
		t.Fatal("expected an error for an unknown flag")
	}
}

func TestAgentEnableDisableWritesFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(buddy.EnvManagedPlist, filepath.Join(dir, "missing.plist"))
	t.Setenv(agent.EnvDisable, "1")

	cfg := &buddy.Config{}
	if err := runAgent(cfg, buddy.Opts{}, []string{clispec.SubEnable}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	path := filepath.Join(dir, "buddy", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "use_agent: true") {
		t.Fatalf("enable wrote:\n%s", data)
	}
	if err := runAgent(cfg, buddy.Opts{}, []string{clispec.SubDisable}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "use_agent: false") {
		t.Fatalf("disable wrote:\n%s", data)
	}

	if err := runAgent(cfg, buddy.Opts{}, []string{clispec.SubForce}); err != nil {
		t.Fatalf("force: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "force_agent: true") {
		t.Fatalf("force wrote:\n%s", data)
	}
	if err := runAgent(cfg, buddy.Opts{}, []string{clispec.SubForce, "--off"}); err != nil {
		t.Fatalf("force --off: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "force_agent: false") {
		t.Fatalf("force --off wrote:\n%s", data)
	}
}

func TestIsAgentEnableDisable(t *testing.T) {
	if !isAgentEnableDisable([]string{clispec.CmdAgent, clispec.SubEnable}) {
		t.Fatal("enable")
	}
	if !isAgentEnableDisable([]string{clispec.CmdAgent, "dis"}) {
		t.Fatal("disable prefix")
	}
	if !isAgentEnableDisable([]string{clispec.CmdAgent, clispec.SubForce}) {
		t.Fatal("force")
	}
	if isAgentEnableDisable([]string{clispec.CmdAgent, clispec.SubStatus}) {
		t.Fatal("status is not a toggle")
	}
	if isAgentEnableDisable([]string{clispec.CmdPolicy, clispec.SubEnable}) {
		t.Fatal("wrong command")
	}
}

func TestAgentStatusRejectsStrayArgs(t *testing.T) {
	t.Setenv(agent.EnvDisable, "1")
	if err := runAgentStatus([]string{"--all"}); err == nil {
		t.Fatal("expected an error for a stray argument")
	}
}

func TestSetupExportJSON(t *testing.T) {
	yamlData := []byte("context:\n  default: sandbox\norgs:\n  acme:\n    environments:\n      sandbox:\n        url: https://example.jamfcloud.com\n")
	path := filepath.Join(t.TempDir(), "export.json")
	if err := writeSetupExportJSON(yamlData, path); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, raw)
	}
	orgs, ok := doc["orgs"].(map[string]any)
	if !ok || orgs["acme"] == nil {
		t.Fatalf("orgs missing: %s", raw)
	}
	if strings.Contains(string(raw), "client_secret") {
		t.Fatalf("export must stay secrets-free:\n%s", raw)
	}
}
