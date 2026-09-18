package main

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

// nestedAgentPath maps Contents/MacOS/<stub> → Contents/Library/LoginItems/BuddyAgent.app.
func nestedAgentPath(executable string) (string, error) {
	exe, err := filepath.EvalSymlinks(executable)
	if err != nil {
		exe = executable
	}
	p := filepath.Join(filepath.Dir(exe), "..", "Library", "LoginItems", "BuddyAgent.app")
	return filepath.Abs(p)
}

func agentSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "Buddy", "agent.sock")
}

func agentReachable() bool {
	sock := agentSocketPath()
	if sock == "" {
		return false
	}
	c, err := net.DialTimeout("unix", sock, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// waitForAgent polls the Agent socket until it answers or the deadline passes.
func waitForAgent(deadline time.Duration) bool {
	if agentReachable() {
		return true
	}
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		time.Sleep(100 * time.Millisecond)
		if agentReachable() {
			return true
		}
	}
	return agentReachable()
}
