package secrets

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// PromptLine reads a single line from stdin (for interactive, non-secret entry).
func PromptLine(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// PromptSecret reads a secret from stdin without echoing it when stdin is a
// terminal. When stdin is not a terminal (piped input), it falls back to a
// plain line read. The prompt is written to stderr so stdout stays clean.
func PromptSecret(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr) // ReadPassword swallows the newline; add one back
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// StdinIsTerminal reports whether stdin is an interactive terminal.
func StdinIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// ReadAllFromReader reads all bytes from r (for --secret-from-stdin).
func ReadAllFromReader(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
