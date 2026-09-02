//go:build darwin && secitem

package secrets

import (
	"fmt"
	"testing"
)

func TestKeychainSecItemDescription(t *testing.T) {
	d := NewKeychainStore().Description()
	want := fmt.Sprintf("macOS Keychain (service %q, Security.framework)", keychainServiceSigned)
	if d != want {
		t.Fatalf("description: %s", d)
	}
}
