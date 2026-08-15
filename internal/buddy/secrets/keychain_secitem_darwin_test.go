//go:build darwin && secitem

package secrets

import (
	"strings"
	"testing"
)

func TestKeychainSecItemDescription(t *testing.T) {
	d := NewKeychainStore().Description()
	if !strings.Contains(d, keychainServiceSigned) || !strings.Contains(d, "Security.framework") {
		t.Fatalf("description: %s", d)
	}
}
