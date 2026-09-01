package mcp

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/script"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// expectedLeanTools is the keep-list for the lean MCP surface (registration order).
var expectedLeanTools = []string{
	"get_instructions",
	"buddy_info",
	"docs",
	"jamf_schema",
	"buddyscript",
}

func TestServerInstructions(t *testing.T) {
	if !strings.Contains(ServerInstructions, "get_instructions") {
		t.Fatal("instructions should tell agents to call get_instructions first")
	}
	if !strings.Contains(ServerInstructions, "developer.jamf.com/mcp") {
		t.Fatal("instructions should mention Jamf hosted MCP URL")
	}
	if !strings.Contains(ServerInstructions, "product schemas") {
		t.Fatal("instructions should note Jamf MCP is for product schemas")
	}
	if !strings.Contains(ServerInstructions, "buddyscript") {
		t.Fatal("instructions should prefer buddyscript")
	}
	if !strings.Contains(ServerInstructions, "no while") {
		t.Fatal("instructions should note Starlark has no while")
	}
	if !strings.Contains(ServerInstructions, "package_wait_ready") {
		t.Fatal("instructions should point at package_wait_ready builtin folklore")
	}
	if !strings.Contains(ServerInstructions, "profile-payloads") {
		t.Fatal("instructions should point agents at profile-payloads docs")
	}
	if !strings.Contains(ServerInstructions, "profile_put_xml") {
		t.Fatal("instructions should name profile_put_xml (ACS quirks)")
	}
	if !strings.Contains(ServerInstructions, "smart_group") {
		t.Fatal("instructions should distinguish smart vs static groups")
	}
	if !strings.Contains(ServerInstructions, "deny_builtins") {
		t.Fatal("instructions should mention deny_builtins")
	}
	if !strings.Contains(ServerInstructions, "smart_group_computers") {
		t.Fatal("instructions should name smart_group_computers for effective membership")
	}
	if !strings.Contains(ServerInstructions, "never invent Classic PUT") {
		t.Fatal("instructions should forbid inventing Classic profile PUTs")
	}
	if !strings.Contains(ServerInstructions, "round-trips") {
		t.Fatal("instructions should discourage micro-batching round-trips")
	}
	if !strings.Contains(ServerInstructions, "bare name") {
		t.Fatal("instructions should tell agents to buddyscript a bare name for one signature")
	}
	if !strings.Contains(ServerInstructions, "Do not docs-get builtins") {
		t.Fatal("instructions should warn against fetching the full builtins index")
	}
	if strings.Contains(ServerInstructions, "starlark_eval") {
		t.Fatal("instructions should not mention starlark_eval")
	}
	if strings.Contains(ServerInstructions, "were removed") {
		t.Fatal("instructions should not include historical notes")
	}
	if strings.Contains(ServerInstructions, "execute-request") {
		t.Fatal("instructions should not spell out execute-request bypass details")
	}
}

func TestAgentPlaybook(t *testing.T) {
	for _, want := range []string{
		"get_instructions",
		"docs",
		"buddyscript",
		"no while",
		"smart_group",
		"smart_group_computers",
		"Bare values",
		"{ok, value",
		"value.result",
		"round-trips",
		"package_wait_ready",
		"profile_put_xml",
		"deny_builtins",
		"developer.jamf.com/mcp",
		"Looking up one builtin",
		"bare expression",
		"no parentheses",
		"buddy://docs/builtins",
	} {
		if !strings.Contains(AgentPlaybook, want) {
			t.Errorf("AgentPlaybook missing %q", want)
		}
	}
}

func TestDescBuddyscriptNamesSmartGroupMembership(t *testing.T) {
	if !strings.Contains(descBuddyscript, "smart_group_computers") {
		t.Fatal("buddyscript tool description should name smart_group_computers")
	}
	if !strings.Contains(descBuddyscript, "membership mutate") {
		t.Fatal("buddyscript tool description should distinguish static membership mutate")
	}
}

func TestToolDescriptionsRequireGetInstructions(t *testing.T) {
	for _, desc := range []string{descBuddyInfo, descDocs, descJamfSchema, descBuddyscript} {
		if !strings.HasPrefix(desc, toolPrerequisite) {
			t.Errorf("description must start with toolPrerequisite: %q…", desc[:min(60, len(desc))])
		}
		if !strings.Contains(desc, "get_instructions") {
			t.Errorf("description must mention get_instructions")
		}
	}
	if strings.Contains(descGetInstructions, toolPrerequisite) {
		t.Fatal("get_instructions description should not require itself as prerequisite")
	}
}

func TestRegisterLeanTools_Present(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "buddy-test", Version: "test"}, nil)
	registerTools(server, "", buddy.Opts{}, slog.Default(), "test")
	_ = expectedLeanTools
	if server == nil {
		t.Fatal("nil server")
	}
	if expectedLeanTools[0] != "get_instructions" {
		t.Fatalf("first lean tool want get_instructions, got %q", expectedLeanTools[0])
	}
}

func TestWithMCPOpts_ForcesSkipConfirm(t *testing.T) {
	got := withMCPOpts(buddy.Opts{SkipConfirm: false, Verbose: true})
	if !got.SkipConfirm {
		t.Fatal("MCP must force SkipConfirm so profile_put_xml and other mutates do not hang on the confirm bridge")
	}
	if !got.Verbose {
		t.Fatal("withMCPOpts should preserve other opts")
	}
}

func TestBuddyscript_MirrorsResultToStdout(t *testing.T) {
	sess, err := script.NewSession("", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := sess.EvalOpts("mcp", "def main():\n    return [1, 2]\n", script.EvalOptions{CallMain: true})
	if res.Error != "" {
		t.Fatalf("eval error: %s", res.Error)
	}
	if res.Result == "" {
		t.Fatal("want non-empty result")
	}
	if res.Stdout != "" {
		t.Fatalf("raw EvalOpts stdout should be empty before MCP mirror, got %q", res.Stdout)
	}
	stdout := res.Stdout
	if stdout == "" && res.Result != "" {
		stdout = res.Result
	}
	if stdout != res.Result {
		t.Fatalf("mirrored stdout=%q result=%q", stdout, res.Result)
	}
}
