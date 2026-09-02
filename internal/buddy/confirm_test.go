package buddy

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestParseConfirmLine(t *testing.T) {
	cases := []struct {
		line       string
		yes, no    string
		defaultYes bool
		answer     bool
		ok         bool
	}{
		{"", "y", "n", false, false, true},
		{"", "y", "n", true, true, true},
		{"y", "y", "n", false, true, true},
		{"YES", "y", "n", false, true, true},
		{"n", "y", "n", true, false, true},
		{"no", "y", "n", false, false, true},
		{"yy", "y", "n", false, false, false},
		{"confirm", "confirm", "abort", false, true, true},
		{"ABORT", "confirm", "abort", true, false, true},
		{"y", "confirm", "abort", false, false, false},
		{"", "confirm", "abort", false, false, true},
	}
	for _, tc := range cases {
		ans, ok := parseConfirmLine(tc.line, tc.yes, tc.no, tc.defaultYes)
		if ok != tc.ok || (ok && ans != tc.answer) {
			t.Errorf("parseConfirmLine(%q, %q, %q, defaultYes=%v) = (%v, %v), want (%v, %v)",
				tc.line, tc.yes, tc.no, tc.defaultYes, ans, ok, tc.answer, tc.ok)
		}
	}
}

func TestConfirmHint(t *testing.T) {
	if g := confirmHint("y", "n", false); g != "[y/N]" {
		t.Fatalf("got %q", g)
	}
	if g := confirmHint("y", "n", true); g != "[Y/n]" {
		t.Fatalf("got %q", g)
	}
	if g := confirmHint("confirm", "abort", false); g != "[confirm/ABORT]" {
		t.Fatalf("got %q", g)
	}
	if g := confirmHint("confirm", "abort", true); g != "[CONFIRM/abort]" {
		t.Fatalf("got %q", g)
	}
}

func TestConfirm_InvalidThenYes(t *testing.T) {
	oldIn, oldOut := confirmIn, confirmOut
	t.Cleanup(func() { confirmIn, confirmOut = oldIn, oldOut })

	confirmIn = strings.NewReader("yy\ny\n")
	var out bytes.Buffer
	confirmOut = &out

	if !Confirm(ConfirmOpts{}, slog.Default()) {
		t.Fatal("want true after yy then y")
	}
	got := out.String()
	if !strings.Contains(got, "Please enter y or n.") {
		t.Fatalf("want re-prompt message, got %q", got)
	}
	if !strings.Contains(got, "[y/N]") {
		t.Fatalf("want [y/N] hint, got %q", got)
	}
}

func TestConfirm_CustomTokensDefaultNo(t *testing.T) {
	oldIn, oldOut := confirmIn, confirmOut
	t.Cleanup(func() { confirmIn, confirmOut = oldIn, oldOut })

	confirmIn = strings.NewReader("\n")
	var out bytes.Buffer
	confirmOut = &out

	if Confirm(ConfirmOpts{Question: "Wipe?", Yes: "confirm", No: "abort"}, slog.Default()) {
		t.Fatal("want false for Enter when default is no")
	}
	if !strings.Contains(out.String(), "Wipe? [confirm/ABORT]:") {
		t.Fatalf("got %q", out.String())
	}
}

func TestConfirm_CustomTokensDefaultYes(t *testing.T) {
	oldIn, oldOut := confirmIn, confirmOut
	t.Cleanup(func() { confirmIn, confirmOut = oldIn, oldOut })

	confirmIn = strings.NewReader("\n")
	var out bytes.Buffer
	confirmOut = &out

	ok := Confirm(ConfirmOpts{
		Question: "Wipe?",
		Yes:      "confirm",
		No:       "abort",
		Default:  "confirm",
	}, slog.Default())
	if !ok {
		t.Fatal("want true for Enter when default is yes token")
	}
	if !strings.Contains(out.String(), "Wipe? [CONFIRM/abort]:") {
		t.Fatalf("got %q", out.String())
	}
}

func TestConfirm_CustomYesTyped(t *testing.T) {
	oldIn, oldOut := confirmIn, confirmOut
	t.Cleanup(func() { confirmIn, confirmOut = oldIn, oldOut })

	confirmIn = strings.NewReader("confirm\n")
	confirmOut = io.Discard

	if !Confirm(ConfirmOpts{Yes: "confirm", No: "abort"}, slog.Default()) {
		t.Fatal("want true")
	}
}

func TestConfirm_BadDefault(t *testing.T) {
	oldIn, oldOut := confirmIn, confirmOut
	t.Cleanup(func() { confirmIn, confirmOut = oldIn, oldOut })
	confirmOut = io.Discard

	if Confirm(ConfirmOpts{Default: "maybe"}, slog.Default()) {
		t.Fatal("want false for invalid default")
	}
}

func TestConfirmPrompt_MessageIsQuestion(t *testing.T) {
	oldIn, oldOut := confirmIn, confirmOut
	t.Cleanup(func() { confirmIn, confirmOut = oldIn, oldOut })

	confirmIn = strings.NewReader("yes\n")
	var out bytes.Buffer
	confirmOut = &out

	if !ConfirmPrompt("Delete stuff?", slog.Default()) {
		t.Fatal("want true")
	}
	got := out.String()
	if !strings.Contains(got, "Delete stuff? [y/N]:") {
		t.Fatalf("want msg as prompt, got %q", got)
	}
	if strings.Count(got, "Proceed?") != 0 {
		t.Fatalf("did not want Proceed?, got %q", got)
	}
}

func TestConfirmPrompt_EmptyDefaultsToProceed(t *testing.T) {
	oldIn, oldOut := confirmIn, confirmOut
	t.Cleanup(func() { confirmIn, confirmOut = oldIn, oldOut })

	confirmIn = strings.NewReader("y\n")
	var out bytes.Buffer
	confirmOut = &out

	if !ConfirmPrompt("", slog.Default()) {
		t.Fatal("want true")
	}
	if !strings.Contains(out.String(), "Proceed? [y/N]:") {
		t.Fatalf("want Proceed? default, got %q", out.String())
	}
}

func TestPushConfirmAsk_UsesHook(t *testing.T) {
	called := false
	restore := PushConfirmAsk(func(opts ConfirmOpts) bool {
		called = true
		return opts.Question == "Hi?"
	})
	defer restore()
	if !Confirm(ConfirmOpts{Question: "Hi?"}, slog.Default()) {
		t.Fatal("want true from hook")
	}
	if !called {
		t.Fatal("hook not called")
	}
}

func TestNormalized_DefaultTracksNo(t *testing.T) {
	_, yes, no, def, defaultYes, err := ConfirmOpts{Yes: "confirm", No: "abort"}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if yes != "confirm" || no != "abort" || def != "abort" || defaultYes {
		t.Fatalf("got yes=%q no=%q def=%q defaultYes=%v", yes, no, def, defaultYes)
	}
}
