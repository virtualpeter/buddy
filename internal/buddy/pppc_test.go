package buddy

import (
	"testing"
)

func TestPPPCFDA(t *testing.T) {
	e := PPPCFDA("com.example.app", `identifier "com.example.app" and anchor apple generic`, "FDA for Example")
	if e["Identifier"] != "com.example.app" {
		t.Fatalf("Identifier=%v", e["Identifier"])
	}
	if e["IdentifierType"] != "bundleID" {
		t.Fatalf("IdentifierType=%v", e["IdentifierType"])
	}
	if e["CodeRequirement"] == "" {
		t.Fatal("missing CodeRequirement")
	}
	if e["Allowed"] != true {
		t.Fatalf("Allowed=%v", e["Allowed"])
	}
	if e["Comment"] != "FDA for Example" {
		t.Fatalf("Comment=%v", e["Comment"])
	}
}

func TestPPPCPayload(t *testing.T) {
	entry := PPPCFDA("com.example.app", "identifier \"com.example.app\"", "")
	services := map[string]interface{}{
		"SystemPolicyAllFiles": []interface{}{entry},
	}
	p, err := PPPCPayload(services, PayloadMetaOpts{PayloadIdentifier: "com.example.pppc"})
	if err != nil {
		t.Fatal(err)
	}
	if p["PayloadType"] != "com.apple.TCC.configuration-profile-policy" {
		t.Fatalf("PayloadType=%v", p["PayloadType"])
	}
	if p["PayloadIdentifier"] != "com.example.pppc" {
		t.Fatalf("PayloadIdentifier=%v", p["PayloadIdentifier"])
	}
	svc, ok := p["Services"].(map[string]interface{})
	if !ok {
		t.Fatalf("Services type %T", p["Services"])
	}
	if _, ok := svc["SystemPolicyAllFiles"]; !ok {
		t.Fatalf("missing SystemPolicyAllFiles: %#v", svc)
	}
}

func TestPrefsPayload(t *testing.T) {
	p, err := PrefsPayload("com.example.app", map[string]interface{}{"Foo": "bar"}, PayloadMetaOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if p["PayloadType"] != "com.example.app" {
		t.Fatalf("PayloadType=%v", p["PayloadType"])
	}
	if p["Foo"] != "bar" {
		t.Fatalf("Foo=%v", p["Foo"])
	}
	if p["PayloadUUID"] == "" {
		t.Fatal("missing PayloadUUID")
	}
}
