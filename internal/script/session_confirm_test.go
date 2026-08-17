package script

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"buddy/internal/buddy"
)

func TestSessionConfirmBridge_BrowserAsk(t *testing.T) {
	s, err := NewSession("", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan bool, 1)
	go func() {
		restore := s.pushConfirmAsk()
		defer restore()
		done <- buddy.Confirm(buddy.ConfirmOpts{Question: "Wipe?", Yes: "confirm", No: "abort"}, slog.Default())
	}()

	deadline := time.After(2 * time.Second)
	for {
		if p := s.PendingConfirm(); p != nil {
			if p.Question != "Wipe?" || p.Yes != "confirm" || p.No != "abort" {
				t.Fatalf("pending=%+v", p)
			}
			if err := s.AnswerConfirm("confirm"); err != nil {
				t.Fatal(err)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for pending confirm")
		case <-time.After(10 * time.Millisecond):
		}
	}

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("want true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Confirm to return")
	}
}

// Without SkipConfirm, Session.Eval installs the browser confirm bridge. MCP has
// no AnswerConfirm path, so Classic mutates (e.g. profile_put_xml) hang until the
// host times out — mcp.Run forces SkipConfirm to avoid this.
func TestSessionConfirmBridge_ProfilePutXMLPendingWithoutSkip(t *testing.T) {
	s, err := NewSession("", buddy.Opts{SkipConfirm: false}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan EvalResult, 1)
	go func() {
		done <- s.EvalOpts("mcp", `def main():
    profile_put_xml(id=1, xml_str="<os_x_configuration_profile></os_x_configuration_profile>")
    return "done"
`, EvalOptions{CallMain: true})
	}()

	deadline := time.After(2 * time.Second)
	for s.PendingConfirm() == nil {
		select {
		case <-deadline:
			t.Fatal("expected pending confirm from profile_put_xml when SkipConfirm is false")
		case <-time.After(5 * time.Millisecond):
		}
	}
	p := s.PendingConfirm()
	if p == nil || !strings.Contains(p.Question, "PUT configuration profile") {
		t.Fatalf("pending confirm=%+v", p)
	}
	if err := s.AnswerConfirm("n"); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.Error != "" {
			t.Fatalf("eval error after cancel: %s", r.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for eval after answering confirm")
	}
}

func TestSessionConfirmBridge_ProfilePutXMLSkipsWithSkipConfirm(t *testing.T) {
	s, err := NewSession("", buddy.Opts{SkipConfirm: true}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan EvalResult, 1)
	go func() {
		// Will fail at Jamf (no cred / bad id) — but must not leave a pending confirm.
		done <- s.EvalOpts("mcp", `def main():
    profile_put_xml(id=1, xml_str="<os_x_configuration_profile></os_x_configuration_profile>")
    return "done"
`, EvalOptions{CallMain: true})
	}()

	select {
	case r := <-done:
		if s.PendingConfirm() != nil {
			t.Fatal("SkipConfirm=true must not install a pending confirm")
		}
		if r.Error == "" && r.Result == "done" {
			t.Fatal("unexpected success talking to Jamf in unit test")
		}
	case <-time.After(3 * time.Second):
		if s.PendingConfirm() != nil {
			t.Fatal("hung on confirm bridge despite SkipConfirm=true")
		}
		t.Fatal("timed out; possible hang on HTTP without timeout")
	}
}

func TestSessionConfirmBridge_DefaultNo(t *testing.T) {
	s, err := NewSession("", buddy.Opts{}, slog.Default(), false)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan bool, 1)
	go func() {
		restore := s.pushConfirmAsk()
		defer restore()
		done <- buddy.Confirm(buddy.ConfirmOpts{Question: "Go?"}, slog.Default())
	}()

	deadline := time.After(2 * time.Second)
	for s.PendingConfirm() == nil {
		select {
		case <-deadline:
			t.Fatal("timeout")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if err := s.AnswerConfirm(""); err != nil { // empty → default no
		t.Fatal(err)
	}
	select {
	case ok := <-done:
		if ok {
			t.Fatal("want false for default no")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
