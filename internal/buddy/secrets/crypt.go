package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

const (
	EnvelopeVersion     = 1
	EnvelopeVersionRaw  = 2
	EnvelopeKDF         = "pbkdf2-sha256"
	EnvelopeKDFRaw      = "raw-aes-gcm"
	EnvelopeSaltLen     = 16
	EnvelopeKeyLen      = 32
	DefaultEnvelopeIter = 600_000

	KindCredBackup = "cred-backup"
	AADCredBackup  = "buddy-cred-backup-v1"

	KindSecretVault = "secret-vault"
	AADSecretVault  = "buddy-secret-vault-v1"

	KindSecretVaultWrap = "secret-vault-wrap"
	AADSecretVaultWrap  = "buddy-secret-vault-wrap-v1"
)

// EnvelopeIter is the PBKDF2 iteration count for new envelopes (tests may lower it).
var EnvelopeIter = DefaultEnvelopeIter

// Envelope is the on-disk JSON wrapper (no secrets in plaintext).
type Envelope struct {
	Buddy string    `json:"buddy"`
	V     int       `json:"v"`
	KDF   string    `json:"kdf"`
	Iter  int       `json:"iter,omitempty"`
	Salt  string    `json:"salt,omitempty"`
	Data  string    `json:"data"` // base64(nonce || ciphertext)
	Wrap  *Envelope `json:"wrap,omitempty"`
}

// EncryptEnvelope seals plain with AES-256-GCM using a PBKDF2-SHA256 key.
func EncryptEnvelope(plain []byte, passphrase, kind, aad string) (Envelope, error) {
	if kind == "" || aad == "" {
		return Envelope{}, fmt.Errorf("envelope kind and aad are required")
	}
	iter := EnvelopeIter
	if iter < 1000 || iter > 10_000_000 {
		return Envelope{}, fmt.Errorf("invalid kdf iteration count")
	}
	salt := make([]byte, EnvelopeSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return Envelope{}, fmt.Errorf("salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, salt, iter, EnvelopeKeyLen)
	if err != nil {
		return Envelope{}, fmt.Errorf("kdf: %w", err)
	}
	data, err := sealGCM(key, plain, aad)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Buddy: kind,
		V:     EnvelopeVersion,
		KDF:   EnvelopeKDF,
		Iter:  iter,
		Salt:  base64.StdEncoding.EncodeToString(salt),
		Data:  data,
	}, nil
}

// EncryptRaw seals plain with AES-256-GCM using a raw 32-byte key (no KDF).
func EncryptRaw(plain, key []byte, kind, aad string) (Envelope, error) {
	if kind == "" || aad == "" {
		return Envelope{}, fmt.Errorf("envelope kind and aad are required")
	}
	if len(key) != EnvelopeKeyLen {
		return Envelope{}, fmt.Errorf("raw key must be %d bytes", EnvelopeKeyLen)
	}
	data, err := sealGCM(key, plain, aad)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Buddy: kind,
		V:     EnvelopeVersionRaw,
		KDF:   EnvelopeKDFRaw,
		Data:  data,
	}, nil
}

func sealGCM(key, plain []byte, aad string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plain, []byte(aad))
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptEnvelope opens an envelope produced by EncryptEnvelope.
// noun is used in error text (e.g. "backup", "vault").
func DecryptEnvelope(raw []byte, passphrase, kind, aad, noun string) ([]byte, error) {
	if noun == "" {
		noun = "envelope"
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("not a buddy %s (parse): %w", noun, err)
	}
	if env.Buddy != kind || env.V != EnvelopeVersion {
		return nil, fmt.Errorf("not a buddy %s (buddy=%q v=%d)", noun, env.Buddy, env.V)
	}
	if env.KDF != EnvelopeKDF {
		return nil, fmt.Errorf("unsupported kdf %q", env.KDF)
	}
	if env.Iter < 1000 || env.Iter > 10_000_000 {
		return nil, fmt.Errorf("invalid kdf iteration count")
	}
	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil || len(salt) < 8 {
		return nil, fmt.Errorf("invalid salt")
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, salt, env.Iter, EnvelopeKeyLen)
	if err != nil {
		return nil, fmt.Errorf("kdf: %w", err)
	}
	plain, err := openGCM(key, env.Data, aad)
	if err != nil {
		return nil, fmt.Errorf("wrong passphrase or corrupt %s", noun)
	}
	return plain, nil
}

// DecryptRaw opens an envelope produced by EncryptRaw.
func DecryptRaw(raw []byte, key []byte, kind, aad, noun string) ([]byte, error) {
	if noun == "" {
		noun = "envelope"
	}
	env, err := ParseEnvelope(raw)
	if err != nil {
		return nil, fmt.Errorf("not a buddy %s (parse): %w", noun, err)
	}
	return decryptRawEnv(env, key, kind, aad, noun)
}

func decryptRawEnv(env Envelope, key []byte, kind, aad, noun string) ([]byte, error) {
	if env.Buddy != kind || env.V != EnvelopeVersionRaw {
		return nil, fmt.Errorf("not a buddy %s (buddy=%q v=%d)", noun, env.Buddy, env.V)
	}
	if env.KDF != EnvelopeKDFRaw {
		return nil, fmt.Errorf("unsupported kdf %q", env.KDF)
	}
	if len(key) != EnvelopeKeyLen {
		return nil, fmt.Errorf("raw key must be %d bytes", EnvelopeKeyLen)
	}
	plain, err := openGCM(key, env.Data, aad)
	if err != nil {
		return nil, fmt.Errorf("wrong key or corrupt %s", noun)
	}
	return plain, nil
}

func openGCM(key []byte, dataB64, aad string) ([]byte, error) {
	sealed, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(sealed) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	plain, err := gcm.Open(nil, sealed[:ns], sealed[ns:], []byte(aad))
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// ParseEnvelope decodes envelope JSON without decrypting.
func ParseEnvelope(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}
