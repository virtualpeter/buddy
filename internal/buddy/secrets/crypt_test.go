package secrets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncryptDecryptEnvelopeRoundTrip(t *testing.T) {
	prev := EnvelopeIter
	EnvelopeIter = 10_000
	t.Cleanup(func() { EnvelopeIter = prev })

	env, err := EncryptEnvelope([]byte(`{"hello":"world"}`), "pw", KindCredBackup, AADCredBackup)
	if err != nil {
		t.Fatal(err)
	}
	if env.Buddy != KindCredBackup || env.V != EnvelopeVersion {
		t.Fatalf("envelope %+v", env)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecryptEnvelope(raw, "pw", KindCredBackup, AADCredBackup, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"hello":"world"}` {
		t.Fatalf("plain = %s", plain)
	}
}

func TestEncryptDecryptRawRoundTrip(t *testing.T) {
	key := make([]byte, EnvelopeKeyLen)
	for i := range key {
		key[i] = byte(i + 1)
	}
	env, err := EncryptRaw([]byte("secret"), key, KindSecretVault, AADSecretVault)
	if err != nil {
		t.Fatal(err)
	}
	if env.V != EnvelopeVersionRaw || env.KDF != EnvelopeKDFRaw || env.Iter != 0 {
		t.Fatalf("envelope %+v", env)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecryptRaw(raw, key, KindSecretVault, AADSecretVault, "vault")
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "secret" {
		t.Fatalf("plain = %s", plain)
	}
	other := make([]byte, EnvelopeKeyLen)
	other[0] = 9
	_, err = DecryptRaw(raw, other, KindSecretVault, AADSecretVault, "vault")
	if err == nil || !strings.Contains(err.Error(), "wrong key") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseDEK(t *testing.T) {
	key := make([]byte, EnvelopeKeyLen)
	key[0] = 0xab
	enc, err := EncodeDEK(key)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ParseDEK(enc)
	if !ok || len(got) != EnvelopeKeyLen || got[0] != 0xab {
		t.Fatalf("ParseDEK(%q) = %v %v", enc, got, ok)
	}
	if _, ok := ParseDEK("hunter2"); ok {
		t.Fatal("passphrase must not parse as DEK")
	}
}

func TestDecryptEnvelopeWrongPassphrase(t *testing.T) {
	prev := EnvelopeIter
	EnvelopeIter = 10_000
	t.Cleanup(func() { EnvelopeIter = prev })

	env, err := EncryptEnvelope([]byte("secret"), "right", KindSecretVault, AADSecretVault)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecryptEnvelope(raw, "wrong", KindSecretVault, AADSecretVault, "vault")
	if err == nil || !strings.Contains(err.Error(), "wrong passphrase") {
		t.Fatalf("err = %v", err)
	}
}

func TestDecryptEnvelopeRejectsOtherKind(t *testing.T) {
	prev := EnvelopeIter
	EnvelopeIter = 10_000
	t.Cleanup(func() { EnvelopeIter = prev })

	env, err := EncryptEnvelope([]byte("secret"), "pw", KindCredBackup, AADCredBackup)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecryptEnvelope(raw, "pw", KindSecretVault, AADSecretVault, "vault")
	if err == nil || !strings.Contains(err.Error(), "not a buddy vault") {
		t.Fatalf("err = %v", err)
	}
}
