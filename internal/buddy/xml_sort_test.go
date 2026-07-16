package buddy

import (
	"strings"
	"testing"
)

func TestXMLSortByKeyAscDesc(t *testing.T) {
	in := `<root><items>
  <item><name>c</name></item>
  <item><name>a</name></item>
  <item><name>b</name></item>
</items></root>`
	got, err := XMLSort(in, XMLSortOpts{Element: "items", Key: "name", Order: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	a := strings.Index(got, ">a</name>")
	b := strings.Index(got, ">b</name>")
	c := strings.Index(got, ">c</name>")
	if !(a < b && b < c) {
		t.Fatalf("expected a,b,c order in %s", got)
	}

	gotDesc, err := XMLSort(in, XMLSortOpts{Element: "items", Key: "name", Order: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	a = strings.Index(gotDesc, ">a</name>")
	b = strings.Index(gotDesc, ">b</name>")
	c = strings.Index(gotDesc, ">c</name>")
	if !(c < b && b < a) {
		t.Fatalf("expected c,b,a order in %s", gotDesc)
	}
}

func TestXMLSortByAttr(t *testing.T) {
	in := `<root>
  <e id="3"/>
  <e id="1"/>
  <e id="2"/>
</root>`
	got, err := XMLSort(in, XMLSortOpts{Attr: "id", Numeric: true})
	if err != nil {
		t.Fatal(err)
	}
	i1 := strings.Index(got, `id="1"`)
	i2 := strings.Index(got, `id="2"`)
	i3 := strings.Index(got, `id="3"`)
	if !(i1 < i2 && i2 < i3) {
		t.Fatalf("numeric attr order: %s", got)
	}
}

func TestXMLSortOnlyTag(t *testing.T) {
	in := `<root>
  <keep>z</keep>
  <item><name>b</name></item>
  <keep>y</keep>
  <item><name>a</name></item>
</root>`
	got, err := XMLSort(in, XMLSortOpts{Only: "item", Key: "name"})
	if err != nil {
		t.Fatal(err)
	}
	// keep elements stay in place; items reorder in their slots.
	k1 := strings.Index(got, "<keep>z</keep>")
	k2 := strings.Index(got, "<keep>y</keep>")
	ia := strings.Index(got, ">a</name>")
	ib := strings.Index(got, ">b</name>")
	if k1 < 0 || k2 < 0 || ia < 0 || ib < 0 {
		t.Fatalf("missing pieces: %s", got)
	}
	if !(k1 < ia && ia < k2 && k2 < ib) {
		t.Fatalf("expected keep z, item a, keep y, item b; got %s", got)
	}
}

func TestXMLSortPlistKey(t *testing.T) {
	in := `<plist><array>
  <dict><key>PayloadIdentifier</key><string>com.b</string></dict>
  <dict><key>PayloadIdentifier</key><string>com.a</string></dict>
</array></plist>`
	got, err := XMLSort(in, XMLSortOpts{Element: "array", Key: "PayloadIdentifier", Only: "dict"})
	if err != nil {
		t.Fatal(err)
	}
	ia := strings.Index(got, "com.a")
	ib := strings.Index(got, "com.b")
	if !(ia < ib) {
		t.Fatalf("expected com.a before com.b: %s", got)
	}
}

func TestXMLSortIgnoreCase(t *testing.T) {
	in := `<root>
  <item><name>B</name></item>
  <item><name>a</name></item>
</root>`
	got, err := XMLSort(in, XMLSortOpts{Key: "name", IgnoreCase: true})
	if err != nil {
		t.Fatal(err)
	}
	ia := strings.Index(got, ">a</name>")
	ib := strings.Index(got, ">B</name>")
	if !(ia < ib) {
		t.Fatalf("ignore_case: %s", got)
	}
}

func TestXMLSortMissingElement(t *testing.T) {
	_, err := XMLSort(`<root/>`, XMLSortOpts{Element: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}
