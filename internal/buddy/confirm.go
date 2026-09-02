package buddy

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Overridable for tests.
var (
	confirmIn  io.Reader = os.Stdin
	confirmOut io.Writer = os.Stdout
)

// ConfirmOpts customizes an interactive yes/no prompt.
//
// Defaults: Question "Proceed?", Yes "y", No "n", Default = No.
// Enter alone selects Default. The hint capitalizes the default token ([y/N] or [CONFIRM/abort]).
type ConfirmOpts struct {
	Question string // prompt text; empty → "Proceed?"
	Yes      string // accepted yes token; empty → "y"
	No       string // accepted no token; empty → "n"
	Default  string // Enter answer; empty → No. Must match Yes or No (case-insensitive).
	SilentNo bool   // if true, do not print "Aborted." when the answer is no (per-item skip)
}

func (o ConfirmOpts) normalized() (question, yes, no, def string, defaultYes bool, err error) {
	return o.Normalize()
}

// Normalize validates and fills ConfirmOpts defaults.
// Returns question, yes token, no token, default token, whether default is yes, and error.
func (o ConfirmOpts) Normalize() (question, yes, no, def string, defaultYes bool, err error) {
	question = strings.TrimSpace(o.Question)
	if question == "" {
		question = "Proceed?"
	}
	yes = strings.TrimSpace(o.Yes)
	if yes == "" {
		yes = "y"
	}
	no = strings.TrimSpace(o.No)
	if no == "" {
		no = "n"
	}
	if strings.EqualFold(yes, no) {
		return "", "", "", "", false, fmt.Errorf("confirm: yes and no must differ (both %q)", yes)
	}
	def = strings.TrimSpace(o.Default)
	if def == "" {
		def = no
	}
	switch {
	case strings.EqualFold(def, yes):
		return question, yes, no, yes, true, nil
	case strings.EqualFold(def, no):
		return question, yes, no, no, false, nil
	default:
		return "", "", "", "", false, fmt.Errorf("confirm: default %q must be yes (%q) or no (%q)", def, yes, no)
	}
}

func confirmHint(yes, no string, defaultYes bool) string {
	if defaultYes {
		return fmt.Sprintf("[%s/%s]", strings.ToUpper(yes), strings.ToLower(no))
	}
	return fmt.Sprintf("[%s/%s]", strings.ToLower(yes), strings.ToUpper(no))
}

// parseConfirmLine interprets a single confirm line against yes/no tokens.
// ok is false for unrecognized input (caller should re-prompt).
// Empty input selects defaultYes. When yes/no are the single-letter defaults,
// "yes"/"no" are also accepted as aliases.
func parseConfirmLine(line, yes, no string, defaultYes bool) (answer bool, ok bool) {
	s := strings.TrimSpace(line)
	if s == "" {
		return defaultYes, true
	}
	if strings.EqualFold(s, yes) {
		return true, true
	}
	if strings.EqualFold(s, no) {
		return false, true
	}
	// Aliases for the classic single-letter tokens.
	if strings.EqualFold(yes, "y") && strings.EqualFold(s, "yes") {
		return true, true
	}
	if strings.EqualFold(no, "n") && strings.EqualFold(s, "no") {
		return false, true
	}
	return false, false
}

// Confirm asks a yes/no question per opts and returns true for yes.
func Confirm(opts ConfirmOpts, logger *slog.Logger) bool {
	if logger == nil {
		logger = GetLogger()
	}
	if ask := currentConfirmAsk(); ask != nil {
		return ask(opts)
	}
	question, yes, no, _, defaultYes, err := opts.normalized()
	if err != nil {
		logger.Error("Invalid confirm options", "error", err)
		fmt.Fprintln(confirmOut, "Aborted.")
		return false
	}
	hint := confirmHint(yes, no, defaultYes)
	reader := bufio.NewReader(confirmIn)
	for {
		fmt.Fprintf(confirmOut, "%s %s: ", question, hint)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			logger.Error("Failed to read input", "error", err)
			return false
		}
		trimmed := strings.TrimSpace(line)
		// Bare EOF (no Enter) always cancels; Enter alone uses the default.
		if trimmed == "" && err == io.EOF {
			logger.Debug("User cancelled")
			fmt.Fprintln(confirmOut, "Aborted.")
			return false
		}
		ans, valid := parseConfirmLine(trimmed, yes, no, defaultYes)
		if !valid {
			fmt.Fprintf(confirmOut, "Please enter %s or %s.\n", yes, no)
			if err == io.EOF {
				logger.Debug("User cancelled")
				fmt.Fprintln(confirmOut, "Aborted.")
				return false
			}
			continue
		}
		if !ans {
			logger.Debug("User cancelled")
			if !opts.SilentNo {
				fmt.Fprintln(confirmOut, "Aborted.")
			}
			return false
		}
		return true
	}
}

// AskYesNo prints "<question> [y/N]: " (or [Y/n] when defaultYes) using classic y/n tokens.
func AskYesNo(question string, defaultYes bool, logger *slog.Logger) bool {
	def := "n"
	if defaultYes {
		def = "y"
	}
	return Confirm(ConfirmOpts{Question: question, Default: def}, logger)
}

// ConfirmPrompt asks msg as the y/n question (default "Proceed?") with Enter = no.
func ConfirmPrompt(msg string, logger *slog.Logger) bool {
	return Confirm(ConfirmOpts{Question: msg}, logger)
}

// ConfirmPromptDefault is like ConfirmPrompt but Enter alone answers yes when defaultYes.
func ConfirmPromptDefault(msg string, defaultYes bool, logger *slog.Logger) bool {
	def := "n"
	if defaultYes {
		def = "y"
	}
	return Confirm(ConfirmOpts{Question: msg, Default: def}, logger)
}

// confirmDangerous prompts for a dangerous action. For ERASE, requireType is "erase" and the user must type "erase".
// For LOCK or other actions, requireType is empty and it behaves like ConfirmPrompt.
func confirmDangerous(msg, requireType string, logger *slog.Logger) bool {
	if msg != "" {
		fmt.Fprintf(confirmOut, "%s\n\n", msg)
	}
	if requireType == "" {
		return Confirm(ConfirmOpts{}, logger)
	}
	if logger == nil {
		logger = GetLogger()
	}
	fmt.Fprintf(confirmOut, "Type %q to confirm: ", requireType)
	reader := bufio.NewReader(confirmIn)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		logger.Error("Failed to read input", "error", err)
		return false
	}
	if strings.TrimSpace(line) != requireType {
		logger.Debug("User did not type required confirmation")
		fmt.Fprintln(confirmOut, "Aborted.")
		return false
	}
	return true
}
