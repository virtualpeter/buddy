package serve

import (
	"html"
	"strings"
	"testing"
)

func TestChromeHeaderSlots(t *testing.T) {
	h := chromeHeader("notebook",
		`<label>tenant <select id="tenant"></select></label><label>user <select id="auth"></select></label>`,
		`<button id="addCell">Add cell</button>`,
		`<button id="runAll" class="primary">Run all</button>`)
	for _, want := range []string{
		`class="app-header"`,
		`href="/docs/"`,
		`href="/notebook"`,
		`href="/ide"`,
		`target="buddy-docs"`,
		`target="buddy-notebook"`,
		`target="buddy-ide"`,
		`window.name="buddy-notebook"`,
		`class="ver"`,
		`class="active">notebook</a>`,
		`id="tenant"`,
		`id="auth"`,
		`id="addCell"`,
		`id="runAll"`,
		`id="theme-dark"`,
		`theme-toggle`,
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("header missing %q\n%s", want, h)
		}
	}
	// Order: brand/nav, session, actions, spacer, run, dark
	idxNav := strings.Index(h, `class="app-nav"`)
	idxTenant := strings.Index(h, `id="tenant"`)
	idxAdd := strings.Index(h, `id="addCell"`)
	idxSpacer := strings.Index(h, `class="spacer"`)
	idxRun := strings.Index(h, `id="runAll"`)
	idxDark := strings.Index(h, `id="theme-dark"`)
	if !(idxNav < idxTenant && idxTenant < idxAdd && idxAdd < idxSpacer && idxSpacer < idxRun && idxRun < idxDark) {
		t.Fatalf("slot order wrong: nav=%d tenant=%d add=%d spacer=%d run=%d dark=%d",
			idxNav, idxTenant, idxAdd, idxSpacer, idxRun, idxDark)
	}
}

func TestChromeNamedTabs(t *testing.T) {
	for _, surface := range []string{"docs", "notebook", "ide"} {
		h := chromeHeader(surface, "", "", "")
		wantName := `window.name="buddy-` + surface + `"`
		if !strings.Contains(h, wantName) {
			t.Errorf("%s header missing %q", surface, wantName)
		}
		for _, id := range []string{"docs", "notebook", "ide"} {
			want := `target="buddy-` + id + `"`
			if !strings.Contains(h, want) {
				t.Errorf("%s header missing %q", surface, want)
			}
		}
	}
	if !strings.Contains(notebookHTML, `target="buddy-notebook"`) {
		t.Error("notebook HTML missing named notebook target")
	}
	if !strings.Contains(ideHTML, `target="buddy-ide"`) {
		t.Error("ide HTML missing named ide target")
	}
	if !strings.Contains(notebookHTML, `window.name="buddy-notebook"`) {
		t.Error("notebook HTML should claim window.name")
	}
	if !strings.Contains(ideHTML, `window.name="buddy-ide"`) {
		t.Error("ide HTML should claim window.name")
	}
}

func TestChromeVersion(t *testing.T) {
	prev := chromeVersion
	t.Cleanup(func() {
		setChromeVersion(prev)
		ideHTML = buildIDEHTML()
		notebookHTML = buildNotebookHTML()
	})
	setChromeVersion("1.2.3-test")
	h := chromeHeader("docs", "", "", "")
	if !strings.Contains(h, `class="ver"`) || !strings.Contains(h, "1.2.3-test") {
		t.Fatalf("header missing version: %s", h)
	}
	setChromeVersion("")
	h2 := chromeHeader("docs", "", "", "")
	if !strings.Contains(h2, ">dev</span>") {
		t.Fatalf("empty version should fall back to dev: %s", h2)
	}
	setChromeVersion(`<script>x</script>`)
	h3 := chromeHeader("docs", "", "", "")
	if strings.Contains(h3, "<script>x</script>") || !strings.Contains(h3, html.EscapeString(`<script>x</script>`)) {
		// EscapeString of that is &lt;script&gt;...
		if !strings.Contains(h3, "&lt;script&gt;") {
			t.Fatalf("version should be HTML-escaped: %s", h3)
		}
	}
}

func TestNotebookIDESharedChrome(t *testing.T) {
	for name, html := range map[string]string{
		"notebook": notebookHTML,
		"ide":      ideHTML,
	} {
		for _, want := range []string{
			`buddy-serve-theme`,
			`buddyServeStore`,
			`buddyServeSoftTab`,
			`prefers-color-scheme`,
			`id="theme-dark"`,
			`class="app-header"`,
			`class="app-nav"`,
			`id="tenant"`,
			`id="auth"`,
			`data-theme`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s missing %q", name, want)
			}
		}
	}
	if !strings.Contains(notebookHTML, `id="runAll"`) {
		t.Error("notebook missing runAll")
	}
	if !strings.Contains(notebookHTML, `buddy-serve-notebook-draft`) {
		t.Error("notebook missing draft key")
	}
	if !strings.Contains(ideHTML, `id="btn-run"`) {
		t.Error("ide missing btn-run")
	}
	if !strings.Contains(ideHTML, `buddy-serve-ide-draft`) {
		t.Error("ide missing draft key")
	}
	// Run should be in header (before runbar for IDE), not only in runbar
	runIdx := strings.Index(ideHTML, `id="btn-run"`)
	runbarIdx := strings.Index(ideHTML, `class="runbar"`)
	if runIdx < 0 || runbarIdx < 0 || runIdx > runbarIdx {
		t.Fatalf("IDE Run should appear in header before runbar (run=%d runbar=%d)", runIdx, runbarIdx)
	}
	if strings.Contains(ideHTML, `class="runbar"`) && strings.Count(ideHTML, `id="btn-run"`) != 1 {
		t.Error("expected single btn-run")
	}
}

func TestDocsTemplateHasChrome(t *testing.T) {
	if !strings.Contains(docsTmplSrc, "{{chromeHeader}}") {
		t.Fatal("docs template missing chromeHeader")
	}
	if !strings.Contains(docsTmplSrc, "{{chromeHead}}") {
		t.Fatal("docs template missing chromeHead")
	}
	if !strings.Contains(docsTmplSrc, "buddy-serve-docs-last") {
		t.Fatal("docs should persist last page id")
	}
	if !strings.Contains(docsTmplSrc, "<title>buddy docs</title>") {
		t.Fatal("docs tab title should stay fixed as buddy docs")
	}
	if strings.Contains(docsTmplSrc, "{{.Title}} · buddy") {
		t.Fatal("docs should not change browser tab title per section")
	}
	if !strings.Contains(docsTmplSrc, "Try it") || !strings.Contains(docsTmplSrc, "buddy-serve-ide-try") {
		t.Fatal("docs should offer Try it → IDE handoff")
	}
	if !strings.Contains(docsTmplSrc, "side-nav") {
		t.Fatal("docs should use side-nav class")
	}
	if strings.Contains(docsTmplSrc, `class="topbar"`) {
		t.Fatal("docs should not keep sidebar topbar")
	}
}

func TestChromePersistAndSoftTab(t *testing.T) {
	head := chromeHeadTags()
	foot := chromeToggleScript()
	for _, want := range []string{
		`buddyServeStore`,
		`buddyServeSoftTab`,
		`buddy-serve-docs-last`,
	} {
		if !strings.Contains(head, want) && !strings.Contains(foot, want) {
			t.Errorf("chrome scripts missing %q", want)
		}
	}
	if !strings.Contains(head, `buddyServeSoftTab`) {
		t.Error("soft tab helper should be in head so page scripts can use it")
	}
	if !strings.Contains(foot, `buddy-serve-docs-last`) {
		t.Error("docs link rewrite should run after header (footer scripts)")
	}
	if !strings.Contains(foot, `window.open('', name)`) {
		t.Error("named tab focus should open('', name) to avoid reloading existing tabs")
	}
	if !strings.Contains(foot, `buddyServeOpenOrFocus`) {
		t.Error("named tab focus should expose buddyServeOpenOrFocus")
	}
	if !strings.Contains(foot, `a[target^="buddy-"]`) {
		t.Error("named tab focus should intercept buddy-* target links")
	}
	if !strings.Contains(ideHTML, `buddy-serve-ide-try`) || !strings.Contains(ideHTML, `buddy-ide-try`) {
		t.Error("ide should consume docs Try it payloads")
	}
}
