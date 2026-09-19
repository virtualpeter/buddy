package serve

import (
	"html"
	"regexp"
	"strings"
	"testing"
)

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

func TestChromaPythonFencePreservesText(t *testing.T) {
	src := "```python\n" +
		"def greet(name):\n" +
		"    print(\"hi\", name)\n" +
		"    # comment\n" +
		"```\n"
	body, err := renderMarkdown(newMarkdown(), src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "chroma light") || strings.Contains(body, "chroma dark") {
		t.Fatalf("WithModeClasses should stay off on chroma 2.27+:\n%s", body)
	}
	i := strings.Index(body, "<pre")
	j := strings.Index(body, "</pre>")
	if i < 0 || j < 0 || j < i {
		t.Fatalf("missing <pre>:\n%s", body)
	}
	inner := body[i:j]
	got := strings.TrimRight(html.UnescapeString(htmlTagRE.ReplaceAllString(inner, "")), "\n")
	want := "def greet(name):\n    print(\"hi\", name)\n    # comment"
	if got != want {
		t.Fatalf("pre textContent changed\nwant %q\ngot  %q\nhtml %s", want, got, body)
	}
}
