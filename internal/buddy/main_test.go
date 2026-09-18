package buddy

import (
	"os"
	"testing"

	"buddy/internal/buddy/agent"
	"buddy/internal/buddy/tokenauth"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "buddy-pkg-tokencache-*")
	if err != nil {
		panic(err)
	}
	// Never dial the developer's own Buddy Agent: a signed one refuses an
	// unsigned test binary, so the suite would pass or fail depending on what
	// is running on the machine. agent_config_test.go opts back in with a fake
	// socket.
	os.Setenv(agent.EnvDisable, "1")
	restore := tokenauth.SetTokenCacheDirForTest(dir)
	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
