package script

import (
	"log/slog"
	"strings"
	"testing"

	"buddy/internal/buddy"
)

const jamfApplePayloadPlist = `<?xml version="1.0"?><plist version="1.0"><dict><key>PayloadContent</key><array>
  <dict>
    <key>PayloadIdentifier</key><string>keep-id</string>
    <key>PayloadType</key><string>com.apple.foo</string>
    <key>PayloadUUID</key><string>keep-uuid</string>
    <key>Value</key><string>old</string>
  </dict>
</array></dict></plist>`

func TestPayloadsListIdentifiersReturnsDicts(t *testing.T) {
	s, err := NewSession("", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := s.Eval("list", `
rows = payloads_list_identifiers("""`+jamfApplePayloadPlist+`""")
rows[0]["identifier"] + "|" + rows[0]["type"] + "|" + rows[0]["uuid"]
`)
	if res.Error != "" {
		t.Fatalf("eval: %#v", res)
	}
	if !strings.Contains(res.Result, "keep-id") || !strings.Contains(res.Result, "com.apple.foo") || !strings.Contains(res.Result, "keep-uuid") {
		t.Fatalf("want identifier|type|uuid, got %#v", res)
	}
}

func TestPayloadPatchRejectsDomainKwarg(t *testing.T) {
	s, err := NewSession("", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := s.Eval("oldkw", `profile_xml_patch_payload(profile_xml_str="<x/>", domain="com.apple.foo", new_payload={})`)
	if res.Error == "" || !strings.Contains(res.Error, "unexpected keyword argument") || !strings.Contains(res.Error, "domain") {
		t.Fatalf("expected unknown domain kwarg, got %#v", res)
	}
}

func TestPayloadsGetMCXDictPayloadContentOnly(t *testing.T) {
	s, err := NewSession("", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	res := s.Eval("mcx", `payloads_get_mcx_dict("""`+jamfApplePayloadPlist+`""", domain="com.apple.foo")`)
	if res.Error == "" || !strings.Contains(res.Error, "payloads_list_identifiers") {
		t.Fatalf("expected PayloadContent-only MCX error, got %#v", res)
	}
}
