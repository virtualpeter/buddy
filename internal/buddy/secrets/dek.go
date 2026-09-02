package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const dekPrefix = "dek:v1:"

// GenerateDEK returns a new 256-bit vault data key.
func GenerateDEK() ([]byte, error) {
	key := make([]byte, EnvelopeKeyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate vault key: %w", err)
	}
	return key, nil
}

// EncodeDEK stores a 32-byte key as a tagged base64 string for MasterStore.
func EncodeDEK(key []byte) (string, error) {
	if len(key) != EnvelopeKeyLen {
		return "", fmt.Errorf("vault key must be %d bytes", EnvelopeKeyLen)
	}
	return dekPrefix + base64.StdEncoding.EncodeToString(key), nil
}

// ParseDEK extracts a 32-byte key from EncodeDEK output.
func ParseDEK(s string) ([]byte, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, dekPrefix) {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, dekPrefix))
	if err != nil || len(raw) != EnvelopeKeyLen {
		return nil, false
	}
	return raw, true
}
