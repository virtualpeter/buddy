package help

import (
	"strings"
	"testing"
)

func TestVersionSectionMarkdown(t *testing.T) {
	md := BuiltinsMarkdown()
	if !strings.Contains(md, "## VERSION — dotted-numeric compare / extract / sort") {
		t.Fatal("VERSION section should be an H2 heading")
	}
	if strings.Contains(md, "offline dotted-numeric") || strings.Contains(md, "Version (offline") {
		t.Fatal("VERSION heading should not call out offline")
	}
	for _, name := range []string{"version_extract", "version_compare", "version_sort"} {
		if !strings.Contains(md, "### `"+name+"`") {
			t.Fatalf("missing ### `%s`", name)
		}
	}
	// Params must stay as prose, not broken into a follow-on code fence.
	idx := strings.Index(md, "### `version_extract`")
	chunk := md[idx:]
	end := strings.Index(chunk, "### `version_compare`")
	if end < 0 {
		t.Fatal("version_compare missing after version_extract")
	}
	chunk = chunk[:end]
	if strings.Contains(chunk, "```\n        extracted") || strings.Contains(chunk, "```\n                extracted") {
		t.Fatalf("version_extract params were split into a code fence:\n%s", chunk)
	}
	if !strings.Contains(chunk, "Returns extracted dotted version, or \"\" if none.") {
		t.Fatalf("expected single-line params prose:\n%s", chunk)
	}

	sections, _ := BuiltinsLibrary()
	found := false
	for _, s := range sections {
		if strings.HasPrefix(s.Title, "VERSION") {
			found = true
			if len(s.Functions) < 3 {
				t.Fatalf("VERSION library section has %d funcs, want >= 3", len(s.Functions))
			}
		}
	}
	if !found {
		t.Fatal("BuiltinsLibrary missing VERSION section")
	}
}
