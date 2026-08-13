package main

import (
	"os"
	"path/filepath"
	"testing"

	"buddy/internal/script"
)

func TestIsKnownCommand(t *testing.T) {
	if !isKnownCommand("policy") {
		t.Error("expected policy to be a known command")
	}
	if isKnownCommand("run") {
		t.Error("run should not be a known command")
	}
	if isKnownCommand("deploy.buddy") {
		t.Error("script filename should not be a known command")
	}
}

func TestIsScriptPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.buddy")
	if err := os.WriteFile(file, []byte("print(1)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if !isScriptPath(file) {
		t.Errorf("expected %q to be a script path", file)
	}
	if isScriptPath(dir) {
		t.Error("directory should not be a script path")
	}
	if isScriptPath(filepath.Join(dir, "missing.buddy")) {
		t.Error("missing file should not be a script path")
	}
}

func TestParseScriptInvocation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantTrace  bool
		wantPath   string
		wantScript []string
	}{
		{
			name:       "path only",
			args:       []string{"deploy.buddy"},
			wantPath:   "deploy.buddy",
			wantScript: nil,
		},
		{
			name:       "trace and path",
			args:       []string{"--trace", "deploy.buddy"},
			wantTrace:  true,
			wantPath:   "deploy.buddy",
			wantScript: nil,
		},
		{
			name:       "path with script args",
			args:       []string{"deploy.buddy", "a", "b"},
			wantPath:   "deploy.buddy",
			wantScript: []string{"a", "b"},
		},
		{
			name:       "path with separator",
			args:       []string{"deploy.buddy", "--", "a", "b"},
			wantPath:   "deploy.buddy",
			wantScript: []string{"a", "b"},
		},
		{
			name:       "trace path separator args",
			args:       []string{"--trace", "deploy.buddy", "--", "dry-run"},
			wantTrace:  true,
			wantPath:   "deploy.buddy",
			wantScript: []string{"dry-run"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTrace, gotPath, gotScript := script.ParseScriptInvocation(tt.args)
			if gotTrace != tt.wantTrace {
				t.Errorf("trace = %v, want %v", gotTrace, tt.wantTrace)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
			}
			if len(gotScript) != len(tt.wantScript) {
				t.Fatalf("script args = %v, want %v", gotScript, tt.wantScript)
			}
			for i := range gotScript {
				if gotScript[i] != tt.wantScript[i] {
					t.Errorf("script args[%d] = %q, want %q", i, gotScript[i], tt.wantScript[i])
				}
			}
		})
	}
}
