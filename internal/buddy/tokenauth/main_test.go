package tokenauth

import (
	"os"
	"testing"

	"buddy/internal/buddy/agent"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "buddy-tokencache-*")
	if err != nil {
		panic(err)
	}
	// Never dial a real Buddy Agent from the suite; agent_token_test.go opts
	// back in with an explicit fake socket.
	os.Setenv(agent.EnvDisable, "1")
	restore := SetTokenCacheDirForTest(dir)
	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
