package buddy

import (
	"strings"
	"testing"
)

func TestBuildProfileCreateXML_ScopeAllAndPayloads(t *testing.T) {
	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>PayloadContent</key><array><dict><key>PayloadIdentifier</key><string>com.example.app</string></dict></array></dict></plist>`
	xml, err := BuildProfileCreateXML(ProfileCreateSpec{
		Name:          "Example Prefs",
		PayloadsPlist: plist,
		ScopeAll:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, "<name>Example Prefs</name>") {
		t.Fatalf("missing name:\n%s", s)
	}
	if !strings.Contains(s, "<all_computers>true</all_computers>") {
		t.Fatalf("expected scope all:\n%s", s)
	}
	if !strings.Contains(s, "&lt;plist") && !strings.Contains(s, "&lt;key&gt;PayloadIdentifier") {
		t.Fatalf("payloads should be entity-encoded:\n%s", s)
	}
	if !strings.Contains(s, "<uuid>") {
		t.Fatalf("missing uuid:\n%s", s)
	}
}

func TestBuildProfileCreateXML_ScopeGroups(t *testing.T) {
	plist := `<plist version="1.0"><dict></dict></plist>`
	xml, err := BuildProfileCreateXML(ProfileCreateSpec{
		Name:          "Scoped",
		PayloadsPlist: plist,
		ScopeAll:      false,
		ScopeGroupIDs: []int{10, 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, "<all_computers>false</all_computers>") {
		t.Fatalf("expected not all computers:\n%s", s)
	}
	if !strings.Contains(s, "<id>10</id>") || !strings.Contains(s, "<id>20</id>") {
		t.Fatalf("missing group ids:\n%s", s)
	}
}

func TestEmptyScopeXML_UnassignedNotAllComputers(t *testing.T) {
	if !strings.Contains(emptyScopeXML, "<all_computers>false</all_computers>") {
		t.Fatal("emptyScopeXML must set all_computers=false (Jamf may treat omitted all_computers as all)")
	}
	if strings.Contains(emptyScopeXML, "<all_computers>true</all_computers>") {
		t.Fatal("emptyScopeXML must not scope all computers")
	}
	if !strings.Contains(emptyScopeXML, "<computer_groups/>") {
		t.Fatal("emptyScopeXML should clear computer_groups")
	}
}

func TestParseProfileIDFromPostBody(t *testing.T) {
	if id := parseProfileIDFromPostBody([]byte(`<os_x_configuration_profile><id>55</id></os_x_configuration_profile>`)); id != 55 {
		t.Fatalf("xml id=%d", id)
	}
	if id := parseProfileIDFromPostBody([]byte(`{"id": 9}`)); id != 9 {
		t.Fatalf("json id=%d", id)
	}
}
