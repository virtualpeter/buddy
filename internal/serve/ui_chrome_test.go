package serve

import (
	"html"
	"strings"
	"testing"
)

func TestChromeHeaderSlots(t *testing.T) {
	h := chromeHeader("ide",
		`<label>tenant <select id="tenant"></select></label><label>user <select id="auth"></select></label>`,
		`<button id="btn-open">Open</button>`,
		`<button id="btn-run" class="primary">Run</button>`)
	for _, want := range []string{
		`class="app-header"`,
		`href="/docs/"`,
		`href="/ide"`,
		`href="/config"`,
		`src="/buddy.png?v=`,
		`target="buddy-docs"`,
		`target="buddy-ide"`,
		`target="buddy-config"`,
		`window.name="buddy-ide"`,
		`class="ver"`,
		`class="active">ide</a>`,
		`id="tenant"`,
		`id="auth"`,
		`id="btn-open"`,
		`id="btn-run"`,
		`id="theme-dark"`,
		`theme-toggle`,
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("header missing %q\n%s", want, h)
		}
	}
	if strings.Contains(h, `href="/notebook"`) {
		t.Fatal("header should not link to /notebook")
	}
	idxNav := strings.Index(h, `class="app-nav"`)
	idxTenant := strings.Index(h, `id="tenant"`)
	idxOpen := strings.Index(h, `id="btn-open"`)
	idxSpacer := strings.Index(h, `class="spacer"`)
	idxRun := strings.Index(h, `id="btn-run"`)
	idxDark := strings.Index(h, `id="theme-dark"`)
	if !(idxNav < idxTenant && idxTenant < idxOpen && idxOpen < idxSpacer && idxSpacer < idxRun && idxRun < idxDark) {
		t.Fatalf("slot order wrong: nav=%d tenant=%d open=%d spacer=%d run=%d dark=%d",
			idxNav, idxTenant, idxOpen, idxSpacer, idxRun, idxDark)
	}
}

func TestChromeNamedTabs(t *testing.T) {
	for _, surface := range []string{"docs", "ide"} {
		h := chromeHeader(surface, "", "", "")
		wantName := `window.name="buddy-` + surface + `"`
		if !strings.Contains(h, wantName) {
			t.Errorf("%s header missing %q", surface, wantName)
		}
		for _, id := range []string{"docs", "ide"} {
			want := `target="buddy-` + id + `"`
			if !strings.Contains(h, want) {
				t.Errorf("%s header missing %q", surface, want)
			}
		}
		if strings.Contains(h, `target="buddy-notebook"`) {
			t.Errorf("%s header still has notebook target", surface)
		}
	}
	if !strings.Contains(ideHTML, `target="buddy-ide"`) {
		t.Error("ide HTML missing named ide target")
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
		if !strings.Contains(h3, "&lt;script&gt;") {
			t.Fatalf("version should be HTML-escaped: %s", h3)
		}
	}
}

func TestIDESharedChrome(t *testing.T) {
	for _, want := range []string{
		`buddy-serve-theme`,
		`buddyServeStore`,
		`buddyServeSoftTab`,
		`prefers-color-scheme`,
		`id="theme-dark"`,
		`class="app-header"`,
		`class="app-nav"`,
		`href="/config"`,
		`src="/buddy.png?v=`,
		`id="tenant"`,
		`id="auth"`,
		`data-theme`,
		`id="btn-run"`,
		`buddy-serve-ide-draft`,
		`id="btn-libs"`,
		`id="btn-copy"`,
		`id="btn-save-out"`,
		`id="btn-stash"`,
		`id="btn-clear-code"`,
		`id="btn-clear-out"`,
		`id="split-h"`,
		`id="libBody"`,
		`data-side="libs"`,
	} {
		if !strings.Contains(ideHTML, want) {
			t.Errorf("ide missing %q", want)
		}
	}
	runIdx := strings.Index(ideHTML, `id="btn-run"`)
	runbarIdx := strings.Index(ideHTML, `class="runbar"`)
	if runIdx < 0 || runbarIdx < 0 || runIdx < runbarIdx {
		t.Fatalf("IDE Run should appear in the runbar (run=%d runbar=%d)", runIdx, runbarIdx)
	}
	if strings.Count(ideHTML, `id="btn-run"`) != 1 {
		t.Error("expected single btn-run")
	}
	if !strings.Contains(ideHTML, `id="btn-clear-code"`) || !strings.Contains(ideHTML, `id="btn-clear-out"`) {
		t.Error("ide missing Clear code/output buttons")
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
	if !strings.Contains(docsTmplSrc, `class="nav-group"`) || !strings.Contains(docsTmplSrc, "<details") {
		t.Fatal("docs sidebar should use collapsible details groups")
	}
	if !strings.Contains(docsTmplSrc, "buddy-serve-docs-nav") {
		t.Fatal("docs should persist sidebar collapse state")
	}
	if !strings.Contains(docsTmplSrc, `eq .ActiveID "cli"`) {
		t.Fatal("docs should offer a command-index page filter")
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
		`rel="icon"`,
		`/buddy.png?v=`,
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
