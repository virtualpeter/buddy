package serve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ideRoot returns the allowlisted filesystem root for /ide open/save.
// BUDDY_IDE_ROOT overrides; otherwise the process working directory.
func ideRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("BUDDY_IDE_ROOT")); v != "" {
		return filepath.Abs(v)
	}
	return os.Getwd()
}

// resolveIDEPath joins rel to the IDE root and ensures the result stays inside the root.
func resolveIDEPath(rel string) (string, error) {
	root, err := ideRoot()
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path required")
	}
	if filepath.IsAbs(rel) {
		// Allow absolute only if under root.
		clean, err := filepath.Abs(rel)
		if err != nil {
			return "", err
		}
		if !pathUnderRoot(root, clean) {
			return "", fmt.Errorf("path outside IDE root %s", root)
		}
		return clean, nil
	}
	clean, err := filepath.Abs(filepath.Join(root, filepath.Clean(rel)))
	if err != nil {
		return "", err
	}
	if !pathUnderRoot(root, clean) {
		return "", fmt.Errorf("path outside IDE root %s", root)
	}
	return clean, nil
}

func pathUnderRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func relToIDERoot(abs string) (string, error) {
	root, err := ideRoot()
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Rel(root, abs)
}
