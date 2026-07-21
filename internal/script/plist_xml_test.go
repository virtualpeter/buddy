package script

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"howett.net/plist"
)

func assertWellFormedXML(t *testing.T, s string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("malformed XML: %v\n---\n%s", err, s)
		}
	}
}

func starlarkDict(t *testing.T, pairs ...string) *starlark.Dict {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatal("pairs must be even")
	}
	d := new(starlark.Dict)
	for i := 0; i < len(pairs); i += 2 {
		if err := d.SetKey(starlark.String(pairs[i]), starlark.String(pairs[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestPlistEncodeDecode_SpecialCharsRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		key  string
		val  string
	}{
		{"ampersand", "k", "a&b"},
		{"angles", "k", "a<b>c"},
		{"quotes", "k", `say "hi" and 'bye'`},
		{"amp_in_key", "a&b", "v"},
		{"lt_in_key", "a<b", "v"},
		{"combo", "PayloadType", "com.example&co <test>"},
		{"cdata_end", "k", "]]>"},
		{"newline", "k", "line1\nline2"},
		{"literal_entities", "k", "a&amp;b&lt;c"},
		{"empty", "k", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := starlarkDict(t, tc.key, tc.val)
			out, err := StarlarkToPlist(d)
			if err != nil {
				t.Fatal(err)
			}
			assertWellFormedXML(t, out)

			// Entity delimiters must appear escaped in the XML text.
			if strings.Contains(tc.val, "&") && !strings.Contains(out, "&amp;") {
				t.Fatalf("expected & escaped in output:\n%s", out)
			}
			if strings.Contains(tc.val, "<") && !strings.Contains(out, "&lt;") {
				t.Fatalf("expected < escaped in output:\n%s", out)
			}

			var raw interface{}
			if err := plist.NewDecoder(strings.NewReader(out)).Decode(&raw); err != nil {
				t.Fatalf("plist library decode: %v", err)
			}
			back, err := PlistDecodeToStarlark(out)
			if err != nil {
				t.Fatal(err)
			}
			bd, ok := back.(*starlark.Dict)
			if !ok {
				t.Fatalf("want dict, got %T", back)
			}
			gotVal, found, err := bd.Get(starlark.String(tc.key))
			if err != nil || !found {
				t.Fatalf("key %q missing after decode", tc.key)
			}
			got, _ := starlark.AsString(gotVal)
			if got != tc.val {
				t.Fatalf("round-trip: got %q want %q", got, tc.val)
			}

			out2, err := StarlarkToPlist(back)
			if err != nil {
				t.Fatal(err)
			}
			assertWellFormedXML(t, out2)
		})
	}
}

func TestPlistEncode_NestedPayloadLike(t *testing.T) {
	thread := &starlark.Thread{Name: "t"}
	v, err := starlark.Eval(thread, "t.star", `
{
  "PayloadContent": [
    {
      "PayloadIdentifier": "com.example&co",
      "PayloadType": "com.example",
      "Enabled": True,
      "Note": "a <b> & \"c\"",
    },
  ],
  "PayloadDisplayName": "Test & Demo",
}
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := StarlarkToPlist(v)
	if err != nil {
		t.Fatal(err)
	}
	assertWellFormedXML(t, out)
	if !strings.Contains(out, "&amp;") || !strings.Contains(out, "&lt;") {
		t.Fatalf("expected entity escapes, got:\n%s", out)
	}
	back, err := PlistDecodeToStarlark(out)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := StarlarkToPlist(back)
	if err != nil {
		t.Fatal(err)
	}
	assertWellFormedXML(t, out2)
}

func TestPlistEncode_ArrayRoot(t *testing.T) {
	thread := &starlark.Thread{Name: "t"}
	v, err := starlark.Eval(thread, "t.star", `["a&b", "<hi>", "]]>", ""]`, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := StarlarkToPlist(v)
	if err != nil {
		t.Fatal(err)
	}
	assertWellFormedXML(t, out)
	back, err := PlistDecodeToStarlark(out)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := back.(*starlark.List)
	if !ok || list.Len() != 4 {
		t.Fatalf("want list of 4, got %v", back)
	}
}
