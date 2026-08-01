package buddy

import (
	"strings"
	"testing"
)

func TestXMLEscapeUnescape(t *testing.T) {
	raw := `<plist><dict><key>a&b</key><string>"x"</string></dict></plist>`
	escaped := XMLEscape(raw)
	if escaped == raw {
		t.Fatal("expected escaping")
	}
	for _, ent := range []string{"&lt;", "&gt;", "&amp;"} {
		if !strings.Contains(escaped, ent) {
			t.Fatalf("missing %q in %q", ent, escaped)
		}
	}
	// Quotes are left alone for element content.
	if !strings.Contains(escaped, `"`) {
		t.Fatalf("expected quotes preserved: %q", escaped)
	}
	round := XMLUnescape(escaped)
	if round != raw {
		t.Fatalf("round-trip: got %q want %q", round, raw)
	}
}

func TestXMLEscape_AmpersandBeforeAngles(t *testing.T) {
	// If < were replaced before &, "&lt;" would become "&amp;lt;".
	got := XMLEscape("<")
	if got != "&lt;" {
		t.Fatalf("got %q", got)
	}
	got = XMLEscape("&<")
	if got != "&amp;&lt;" {
		t.Fatalf("got %q want &amp;&lt;", got)
	}
}

func TestXMLUnescapeNamedAndNumeric(t *testing.T) {
	got := XMLUnescape(`&lt;a&gt;&quot;b&quot;&#39;c&#39;`)
	want := `<a>"b"'c'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestJamfPayloadEmbed_RawAmpInInnerProducesSingleOuterAmp(t *testing.T) {
	// Bug case: inner plist XML with raw & (invalid plist XML but might come from double-unescape).
	inner := `<?xml version="1.0"?><plist version="1.0"><dict><key>k</key><string>a&b</string></dict></plist>`
	outer := xmlEncodePayload(inner)
	if !strings.Contains(outer, "a&amp;b") {
		t.Fatalf("got %q", outer)
	}
	if strings.Contains(outer, "a&amp;amp;b") {
		t.Fatal("raw & should produce single &amp; in outer, not double")
	}
}

func TestJamfPayloadEmbed_ProperInnerAmpProducesDoubleOuterAmp(t *testing.T) {
	inner := `<?xml version="1.0"?><plist version="1.0"><dict><key>k</key><string>a&amp;b</string></dict></plist>`
	outer := xmlEncodePayload(inner)
	if !strings.Contains(outer, "a&amp;amp;b") {
		t.Fatalf("inner &amp; must become &amp;amp; in outer Jamf payloads, got %q", outer)
	}
}
