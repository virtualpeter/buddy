package buddy

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// AppImport writes a desired-state .buddy script for name (from AppGet).
// When outputPath is empty, writes to apps/<slug>.buddy under the current directory.
// Returns the path written.
func AppImport(credName, name, outputPath string, logger *slog.Logger) (string, error) {
	detail, err := AppGet(credName, name, logger)
	if err != nil {
		return "", err
	}
	desired := AppDesiredFromDetail(detail)
	body, err := RenderAppImportScript(desired)
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(outputPath)
	if path == "" {
		slug := normalizeAppSlugKey(desired.Name)
		if slug == "" {
			slug = "app"
		}
		path = filepath.Join("apps", slug+".buddy")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return "", err
	}
	fmt.Printf("Wrote desired-state script for %q (%s) → %s\n", desired.Name, desired.Kind, path)
	fmt.Printf("Edit VERSION if needed, then: buddy %s\n", path)
	return path, nil
}
