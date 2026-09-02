package buddy

import (
	"os"
	"testing"

	"buddy/internal/buddy/tokenauth"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "buddy-pkg-tokencache-*")
	if err != nil {
		panic(err)
	}
	restore := tokenauth.SetTokenCacheDirForTest(dir)
	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
