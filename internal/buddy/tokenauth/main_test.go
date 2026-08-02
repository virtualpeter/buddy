package tokenauth

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "buddy-tokencache-*")
	if err != nil {
		panic(err)
	}
	restore := SetTokenCacheDirForTest(dir)
	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
