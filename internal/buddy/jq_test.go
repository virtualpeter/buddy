package buddy

import (
	"strings"
	"testing"
	"time"
)

func TestApplyJQ_ProjectAndSelect(t *testing.T) {
	got, err := ApplyJQ(map[string]any{"a": 1, "b": 2}, ".a")
	if err != nil {
		t.Fatal(err)
	}
	if classicString(got) != "1" {
		t.Fatalf("project: %#v", got)
	}

	rows := []any{
		map[string]any{"name": "keep", "n": 1},
		map[string]any{"name": "drop", "n": 2},
	}
	got, err = ApplyJQ(rows, `[.[] | select(.name == "keep")]`)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := got.([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("select: %#v", got)
	}
	m, ok := jsonAsMap(list[0])
	if !ok || classicString(m["name"]) != "keep" {
		t.Fatalf("select row: %#v", list[0])
	}
}

func TestApplyJQStream_Multiple(t *testing.T) {
	rows := []any{
		map[string]any{"Computer_Name": "Mac-A"},
		map[string]any{"Computer_Name": "Mac-B"},
	}
	stream, err := ApplyJQStream(rows, ".[].Computer_Name")
	if err != nil {
		t.Fatal(err)
	}
	if len(stream) != 2 || stream[0] != "Mac-A" || stream[1] != "Mac-B" {
		t.Fatalf("stream: %#v", stream)
	}
	collapsed := collapseJQStream(stream)
	list, ok := collapsed.([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("collapse: %#v", collapsed)
	}
}

func TestApplyJQ_ParseError(t *testing.T) {
	_, err := ApplyJQ(map[string]any{"a": 1}, ".foo & .bar")
	if err == nil || !strings.Contains(err.Error(), "jq:") {
		t.Fatalf("parse: %v", err)
	}
}

func TestApplyJQ_EmptyExpr(t *testing.T) {
	_, err := ApplyJQ(nil, "  ")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("empty: %v", err)
	}
}

func TestApplyJQ_Timeout(t *testing.T) {
	old := jqTimeout
	jqTimeout = 80 * time.Millisecond
	defer func() { jqTimeout = old }()
	_, err := ApplyJQ(nil, "repeat(0)")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout: %v", err)
	}
}

func TestPrintJQRaw(t *testing.T) {
	var buf strings.Builder
	if err := printJQRaw(&buf, []any{"Ada", 1.0, map[string]any{"n": "x"}}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "Ada\n") || strings.Contains(got, `"Ada"`) {
		t.Fatalf("string should be unquoted:\n%s", got)
	}
	if !strings.Contains(got, "1\n") {
		t.Fatalf("number:\n%s", got)
	}
	if !strings.Contains(got, `"n":"x"`) && !strings.Contains(got, `"n": "x"`) {
		t.Fatalf("object json:\n%s", got)
	}
}
