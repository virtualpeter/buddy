package secrets

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// PromptLine reads a single line from stdin (for interactive secret entry).
func PromptLine(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ReadAllFromReader reads all bytes from r (for --secret-from-stdin).
func ReadAllFromReader(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
