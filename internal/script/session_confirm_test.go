package script

import (
	"log/slog"
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
