package main

import (
	"path/filepath"
)

// helperPath maps Contents/MacOS/<stub> → Contents/Helpers/buddy.
func helperPath(executable string) (string, error) {
	exe, err := filepath.EvalSymlinks(executable)
	if err != nil {
		exe = executable
	}
	p := filepath.Join(filepath.Dir(exe), "..", "Helpers", "buddy")
	return filepath.Abs(p)
}
