//go:build darwin && !secitem

package secrets

import (
	"strings"
	"testing"
)

func TestKeychainCLIDescription(t *testing.T) {
	d := NewKeychainStore().Description()
	if !strings.Contains(d, keychainServiceCLI) || !strings.Contains(d, "security CLI") {
		t.Fatalf("description: %s", d)
	}
}
