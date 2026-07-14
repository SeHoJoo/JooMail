package httpapi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

var errCredentialUnavailable = errors.New("credential unavailable")

type storedCredential struct {
	Email        string    `json:"email"`
	IMAPUsername string    `json:"imapUsername"`
	Password     string    `json:"password"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// credentialBundle is the on-disk v2 format.  Keeping the account list in the
// session credential file deliberately avoids adding a second account store.
type credentialBundle struct {
	Version  int                `json:"version"`
	Accounts []storedCredential `json:"accounts"`
}

type credentialStore struct {
	dir string
	key []byte
}

func newCredentialStore(config Config) (*credentialStore, error) {
	key, err := parseCredentialKey(config.CredentialKey)
	if err != nil {
		return nil, err
	}
	if config.CredentialDir == "" {
		return nil, errCredentialUnavailable
	}
	return &credentialStore{dir: config.CredentialDir, key: key}, nil
}

func parseCredentialKey(value string) ([]byte, error) {
	if value == "" {
		return nil, errCredentialUnavailable
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if len(value) == 32 {
		return []byte(value), nil
	}
	return nil, errCredentialUnavailable
}

func (s *credentialStore) Save(sessionID string, credential storedCredential) error {
	return s.SaveBundle(sessionID, credentialBundle{Version: 2, Accounts: []storedCredential{credential}})
}

func (s *credentialStore) SaveBundle(sessionID string, bundle credentialBundle) error {
	if bundle.Version != 2 || len(bundle.Accounts) == 0 {
		return errCredentialUnavailable
	}
	for _, credential := range bundle.Accounts {
		if credential.Email == "" || credential.Password == "" {
			return errCredentialUnavailable
		}
	}
	if sessionID == "" {
		return errCredentialUnavailable
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.dir, 0o700); err != nil {
		return err
	}

	plaintext, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)

	path := s.path(sessionID)
	temp, err := os.CreateTemp(s.dir, ".credential-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	if _, err := temp.Write(ciphertext); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return err
	}
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		_ = os.Remove(tempName)
		return err
	}
	return os.Chmod(path, 0o600)
}

func (s *credentialStore) Load(sessionID string, email string) (storedCredential, error) {
	bundle, err := s.LoadBundle(sessionID)
	if err != nil {
		return storedCredential{}, err
	}
	for _, credential := range bundle.Accounts {
		if equalEmail(credential.Email, email) && !time.Now().After(credential.ExpiresAt) {
			return credential, nil
		}
	}
	return storedCredential{}, errCredentialUnavailable
}

func (s *credentialStore) LoadBundle(sessionID string) (credentialBundle, error) {
	var bundle credentialBundle
	ciphertext, err := os.ReadFile(s.path(sessionID))
	if err != nil {
		return bundle, errCredentialUnavailable
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return bundle, errCredentialUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return bundle, errCredentialUnavailable
	}
	if len(ciphertext) < aead.NonceSize() {
		return bundle, errCredentialUnavailable
	}
	nonce := ciphertext[:aead.NonceSize()]
	body := ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		return bundle, errCredentialUnavailable
	}
	if err := json.Unmarshal(plaintext, &bundle); err == nil && bundle.Version == 2 && len(bundle.Accounts) > 0 {
		return bundle, nil
	}
	var legacy storedCredential
	if err := json.Unmarshal(plaintext, &legacy); err != nil || legacy.Email == "" {
		return bundle, errCredentialUnavailable
	}
	return credentialBundle{Version: 2, Accounts: []storedCredential{legacy}}, nil
}

func (s *credentialStore) Delete(sessionID string) error {
	if sessionID == "" {
		return nil
	}
	if err := os.Remove(s.path(sessionID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *credentialStore) path(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}
