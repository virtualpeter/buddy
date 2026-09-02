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

func TestXMLUnescapeNamedAndNumeric(t *testing.T) {
	got := XMLUnescape(`&lt;a&gt;&quot;b&quot;&#39;c&#39;`)
	want := `<a>"b"'c'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
