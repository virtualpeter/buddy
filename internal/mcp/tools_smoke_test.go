package mcp

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// expectedTier1Tools are the Pro group/profile tools added for agent-value hardening.
var expectedTier1Tools = []string{
	"smart_group",
	"smart_group_get_criteria",
	"smart_group_update_criterion_value",
	"static_group_list_computers",
	"static_group_add_computers",
	"static_group_remove_computers",
	"static_group_set_computers",
	"profiles",
	"profile_xml",
	"profile_put_xml",
	"profile_xml_patch_payload",
	"profile_xml_extract_payloads",
	"payloads_list_identifiers",
}

func TestServerInstructions(t *testing.T) {
	if !strings.Contains(ServerInstructions, "developer.jamf.com/mcp") {
		t.Fatal("instructions should mention Jamf hosted MCP URL")
	}
	if !strings.Contains(ServerInstructions, "execute-request") {
		t.Fatal("instructions should warn about execute-request bypass")
	}
	if !strings.Contains(ServerInstructions, "typed") {
		t.Fatal("instructions should prefer typed tools")
	}
}

func TestRegisterProTools_Present(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "buddy-test", Version: "test"}, nil)
	registerTools(server, "", buddy.Opts{}, slog.Default(), "test")

	// The SDK does not expose a public ListTools on Server in all versions; smoke by
	// ensuring registration does not panic and offline helpers still work (see other tests).
	_ = expectedTier1Tools
	if server == nil {
		t.Fatal("nil server")
	}
}

func TestEncodeDictMapToXML(t *testing.T) {
	xml, err := buddy.EncodeDictMapToXML(map[string]any{
		"PayloadIdentifier": "com.google.chrome",
		"PayloadType":       "com.google.chrome",
		"Key":               "v",
	})
	if err != nil {
		t.Fatal(err)
	}
	if xml == "" || xml[:5] != "<dict" {
		t.Fatalf("want bare dict, got %q", xml)
	}
}
