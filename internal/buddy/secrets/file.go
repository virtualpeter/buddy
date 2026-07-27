package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileStore stores secrets as JSON files under the buddy secrets directory.
type FileStore struct{}

// NewFileStore creates a file-backed secret store.
func NewFileStore() (*FileStore, error) {
	dir, err := SecretsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create secrets directory: %w", err)
	}
	return &FileStore{}, nil
}

func (f *FileStore) Description() string {
	dir, err := SecretsDir()
	if err != nil {
		return "file (path unknown)"
	}
	return dir
}

type secretFile struct {
	Password     string `json:"password,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

func (f *FileStore) readFile(credName string) (*secretFile, error) {
	path, err := secretFilePath(credName)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &secretFile{}, nil
		}
		return nil, fmt.Errorf("read secrets file: %w", err)
	}
	var sf secretFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse secrets file: %w", err)
	}
	return &sf, nil
}

func (f *FileStore) writeFile(credName string, sf *secretFile) error {
	path, err := secretFilePath(credName)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}
	data, err := json.Marshal(sf)
	if err != nil {
		return fmt.Errorf("marshal secrets file: %w", err)
	}
	tmp, err := os.CreateTemp(dir, credName+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp secrets file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp secrets file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp secrets file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp secrets file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace secrets file: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func (f *FileStore) Get(credName string, kind Kind) (string, error) {
	sf, err := f.readFile(credName)
	if err != nil {
		return "", err
	}
	val, ok, err := valueFromSecretFile(sf, kind)
	if err != nil {
		return "", err
	}
	if !ok || val == "" {
		return "", fmt.Errorf("secret %q not found in file store for credential %q", kind, credName)
	}
	return val, nil
}

func (f *FileStore) Set(credName string, kind Kind, value string) error {
	sf, err := f.readFile(credName)
	if err != nil {
		return err
	}
	if err := setSecretFileField(sf, kind, value); err != nil {
		return err
	}
	return f.writeFile(credName, sf)
}

func (f *FileStore) Delete(credName string, kind Kind) error {
	sf, err := f.readFile(credName)
	if err != nil {
		return err
	}
	switch kind {
	case KindPassword:
		sf.Password = ""
	case KindClientSecret:
		sf.ClientSecret = ""
	default:
		return fmt.Errorf("unknown secret kind %v", kind)
	}
	if sf.Password == "" && sf.ClientSecret == "" {
		path, err := secretFilePath(credName)
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove secrets file: %w", err)
		}
		return nil
	}
	return f.writeFile(credName, sf)
}

func (f *FileStore) Has(credName string, kind Kind) (bool, error) {
	sf, err := f.readFile(credName)
	if err != nil {
		return false, err
	}
	val, ok, err := valueFromSecretFile(sf, kind)
	if err != nil {
		return false, err
	}
	return ok && val != "", nil
}

func (f *FileStore) List() ([]Entry, error) {
	dir, err := SecretsDir()
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list secrets directory: %w", err)
	}
	var out []Entry
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		authName := strings.TrimSuffix(e.Name(), ".json")
		if authName == "" {
			continue
		}
		sf, err := f.readFile(authName)
		if err != nil {
			return nil, err
		}
		if sf.Password != "" {
			out = append(out, Entry{AuthName: authName, Kind: KindPassword})
		}
		if sf.ClientSecret != "" {
			out = append(out, Entry{AuthName: authName, Kind: KindClientSecret})
		}
	}
	return out, nil
}

func valueFromSecretFile(sf *secretFile, kind Kind) (string, bool, error) {
	switch kind {
	case KindPassword:
		return sf.Password, sf.Password != "", nil
	case KindClientSecret:
		return sf.ClientSecret, sf.ClientSecret != "", nil
	default:
		return "", false, fmt.Errorf("unknown secret kind %v", kind)
	}
}

func setSecretFileField(sf *secretFile, kind Kind, value string) error {
	switch kind {
	case KindPassword:
		sf.Password = value
	case KindClientSecret:
		sf.ClientSecret = value
	default:
		return fmt.Errorf("unknown secret kind %v", kind)
	}
	return nil
}

// ErrNotFound indicates a secret is not present in the store.
var ErrNotFound = errors.New("secret not found")
