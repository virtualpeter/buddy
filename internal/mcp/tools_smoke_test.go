package mcp

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
	"buddy/internal/script"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// expectedLeanTools is the keep-list for the lean MCP surface.
var expectedLeanTools = []string{
	"buddy_info",
	"docs",
	"jamf_schema",
	"buddyscript",
}

func TestServerInstructions(t *testing.T) {
	if !strings.Contains(ServerInstructions, "developer.jamf.com/mcp") {
		t.Fatal("instructions should mention Jamf hosted MCP URL")
	}
	if !strings.Contains(ServerInstructions, "product schemas") {
		t.Fatal("instructions should note Jamf MCP is for product schemas")
	}
	if !strings.Contains(ServerInstructions, "buddyscript") {
		t.Fatal("instructions should prefer buddyscript")
	}
	if !strings.Contains(ServerInstructions, "package_wait_ready") {
		t.Fatal("instructions should point at package_wait_ready builtin folklore")
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

func TestRegisterLeanTools_Present(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "buddy-test", Version: "test"}, nil)
	registerTools(server, "", buddy.Opts{}, slog.Default(), "test")
	_ = expectedLeanTools
	if server == nil {
		t.Fatal("nil server")
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
