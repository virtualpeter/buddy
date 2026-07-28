package serve

import (
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

func TestNotebookIDESharedChrome(t *testing.T) {
	for name, html := range map[string]string{
		"notebook": notebookHTML,
		"ide":      ideHTML,
	} {
		for _, want := range []string{
			`buddy-serve-theme`,
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
	if !strings.Contains(ideHTML, `id="btn-run"`) {
		t.Error("ide missing btn-run")
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
	if !strings.Contains(docsTmplSrc, "side-nav") {
		t.Fatal("docs should use side-nav class")
	}
	if strings.Contains(docsTmplSrc, `class="topbar"`) {
		t.Fatal("docs should not keep sidebar topbar")
	}
}
