package script

import (
	"fmt"
	"strings"
	"sync"

	"buddy/internal/buddy"
)

// PendingConfirm is the JSON shape exposed to the IDE while a confirm waits.
type PendingConfirm struct {
	Question string `json:"question"`
	Yes      string `json:"yes"`
	No       string `json:"no"`
	Default  string `json:"default"`
	Hint     string `json:"hint"`
}

type confirmBridge struct {
	mu      sync.Mutex
	pending *PendingConfirm
	answer  chan string
}

func newConfirmBridge() *confirmBridge {
	return &confirmBridge{}
}

func (b *confirmBridge) snapshot() *PendingConfirm {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		return nil
	}
	cp := *b.pending
	return &cp
}

func (b *confirmBridge) reply(answer string) error {
	if b == nil {
		return fmt.Errorf("no confirm bridge")
	}
	b.mu.Lock()
	ch := b.answer
	pending := b.pending != nil
	b.mu.Unlock()
	if !pending || ch == nil {
		return fmt.Errorf("no confirm pending")
	}
	select {
	case ch <- answer:
		return nil
	default:
		return fmt.Errorf("confirm already answered")
	}
}

func (b *confirmBridge) ask(opts buddy.ConfirmOpts) bool {
	question, yes, no, def, defaultYes, err := opts.Normalize()
	if err != nil {
		return false
	}
	hint := fmt.Sprintf("[%s/%s]", strings.ToLower(yes), strings.ToUpper(no))
	if defaultYes {
		hint = fmt.Sprintf("[%s/%s]", strings.ToUpper(yes), strings.ToLower(no))
	}
	answerCh := make(chan string, 1)
	b.mu.Lock()
	if b.pending != nil {
		b.mu.Unlock()
		return false
	}
	b.pending = &PendingConfirm{
		Question: question,
		Yes:      yes,
		No:       no,
		Default:  def,
		Hint:     hint,
	}
	b.answer = answerCh
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.pending = nil
		b.answer = nil
		b.mu.Unlock()
	}()

	raw := <-answerCh
	return resolveConfirmAnswer(raw, yes, no, defaultYes)
}

func resolveConfirmAnswer(raw, yes, no string, defaultYes bool) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return defaultYes
	}
	if strings.EqualFold(s, yes) {
		return true
	}
	if strings.EqualFold(s, no) {
		return false
	}
	if strings.EqualFold(yes, "y") && strings.EqualFold(s, "yes") {
		return true
	}
	if strings.EqualFold(no, "n") && strings.EqualFold(s, "no") {
		return false
	}
	return false
}
