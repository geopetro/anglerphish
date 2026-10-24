// Package crypto provides AES-256-GCM encryption for sensitive database fields.
// This package is intentionally isolated with only standard library dependencies
// to avoid import cycles with other packages.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	// EncryptionKeyEnvVar is the environment variable name for the encryption key
	EncryptionKeyEnvVar = "ANGLERPHISH_ENCRYPTION_KEY"

	// EncryptedPrefix is the prefix for encrypted values
	EncryptedPrefix = "ENC:v1:"

	// RequiredKeyLength is the required length for AES-256 (32 bytes)
	RequiredKeyLength = 32

	// EncodedKeyLength is the length of a base64url-encoded 32-byte key (no padding)
	EncodedKeyLength = 44
)

// Mode describes how the loaded key (if any) is used.
type Mode int

const (
	// ModeOff means no key is loaded; Encrypt and Decrypt pass plaintext through.
	ModeOff Mode = iota
	// ModeOn means a key is loaded; Encrypt encrypts and Decrypt decrypts.
	ModeOn
	// ModeDecryptOnly means a key is loaded; Encrypt passes through but Decrypt still works.
	ModeDecryptOnly
)

// KeySource describes where the loaded key came from.
type KeySource string

const (
	SourceNone    KeySource = "none"
	SourceEnv     KeySource = "environment"
	SourceSession KeySource = "session"
)

var (
	// ErrNoKeyConfigured is returned when decryption is attempted without a key
	ErrNoKeyConfigured = errors.New("encryption key not configured")

	// ErrInvalidKeyLength is returned when the key length is incorrect
	ErrInvalidKeyLength = errors.New("encryption key must be a 44-character base64url-encoded 32-byte key")

	// ErrDecryptionFailed is returned when decryption fails
	ErrDecryptionFailed = errors.New("decryption failed - invalid ciphertext or wrong key")

	// ErrInvalidEncryptedFormat is returned when the encrypted format is invalid
	ErrInvalidEncryptedFormat = errors.New("invalid encrypted data format")
)

// holder is the process-wide key state, guarded by mu.
var holder = struct {
	mu     sync.RWMutex
	key    []byte
	mode   Mode
	source KeySource
}{source: SourceNone}

// ValidateKey checks that key is a base64url-encoded 32-byte value.
func ValidateKey(key string) error {
	_, err := decodeKey(key)
	return err
}

func decodeKey(key string) ([]byte, error) {
	if len(key) != EncodedKeyLength {
		return nil, ErrInvalidKeyLength
	}
	b, err := base64.URLEncoding.DecodeString(key)
	if err != nil || len(b) != RequiredKeyLength {
		return nil, ErrInvalidKeyLength
	}
	return b, nil
}

// ReadEnvKey returns the raw value of the key environment variable, without validation.
func ReadEnvKey() (key string, present bool) {
	key = os.Getenv(EncryptionKeyEnvVar)
	return key, key != ""
}

// LoadKey validates and loads key, switching to ModeOn.
func LoadKey(key string, source KeySource) error {
	b, err := decodeKey(key)
	if err != nil {
		return err
	}
	holder.mu.Lock()
	defer holder.mu.Unlock()
	holder.key = b
	holder.mode = ModeOn
	holder.source = source
	return nil
}

// SetDecryptOnly stops encrypting new values while keeping the key for decryption.
// It has no effect when no key is loaded.
func SetDecryptOnly() {
	holder.mu.Lock()
	defer holder.mu.Unlock()
	if holder.key != nil {
		holder.mode = ModeDecryptOnly
	}
}

// Unload clears the key and returns to ModeOff.
func Unload() {
	holder.mu.Lock()
	defer holder.mu.Unlock()
	holder.key = nil
	holder.mode = ModeOff
	holder.source = SourceNone
}

// CurrentMode returns the current mode.
func CurrentMode() Mode {
	holder.mu.RLock()
	defer holder.mu.RUnlock()
	return holder.mode
}

// CurrentSource returns where the loaded key came from.
func CurrentSource() KeySource {
	holder.mu.RLock()
	defer holder.mu.RUnlock()
	return holder.source
}

// HasKey returns whether a key is loaded.
func HasKey() bool {
	holder.mu.RLock()
	defer holder.mu.RUnlock()
	return holder.key != nil
}

// IsEncryptionEnabled returns whether new values are currently being encrypted (ModeOn).
func IsEncryptionEnabled() bool {
	return CurrentMode() == ModeOn
}

// GenerateEncryptionKey generates a new random 32-byte encryption key.
// Returns the key as a base64url-encoded string suitable for the environment variable.
// The encoded key is 44 characters long and provides 256 bits of entropy.
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, RequiredKeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %w", err)
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

// IsEncrypted checks if a value is encrypted (has the encryption prefix).
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, EncryptedPrefix)
}

// Encrypt encrypts a plaintext string using AES-256-GCM.
// If encryption is not enabled, returns the plaintext unchanged.
// If the value is already encrypted, returns it unchanged.
func Encrypt(plaintext string) (string, error) {
	// Empty strings don't need encryption
	if plaintext == "" {
		return "", nil
	}

	// Already encrypted, return as-is
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}

	holder.mu.RLock()
	defer holder.mu.RUnlock()

	// Only encrypt in ModeOn
	if holder.mode != ModeOn {
		return plaintext, nil
	}

	gcm, err := newGCM(holder.key)
	if err != nil {
		return "", err
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Encode nonce and ciphertext as base64
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)
	ciphertextB64 := base64.StdEncoding.EncodeToString(ciphertext)

	// Return formatted encrypted value: ENC:v1:nonce:ciphertext
	return fmt.Sprintf("%s%s:%s", EncryptedPrefix, nonceB64, ciphertextB64), nil
}

// Decrypt decrypts an encrypted string using AES-256-GCM.
// If the value is not encrypted (no prefix), returns it unchanged.
// This allows backward compatibility with plaintext values.
func Decrypt(ciphertext string) (string, error) {
	// Empty strings don't need decryption
	if ciphertext == "" {
		return "", nil
	}

	// If not encrypted (no prefix), return as-is (backward compatibility)
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil
	}

	holder.mu.RLock()
	defer holder.mu.RUnlock()

	// Encrypted data requires a loaded key (ModeOn or ModeDecryptOnly)
	if holder.key == nil {
		return "", ErrNoKeyConfigured
	}
	return openWithKey(ciphertext, holder.key)
}

// DecryptWithKey decrypts ciphertext with a candidate key without loading it.
// Values without the encryption prefix are returned unchanged.
func DecryptWithKey(ciphertext, key string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil
	}
	b, err := decodeKey(key)
	if err != nil {
		return "", err
	}
	return openWithKey(ciphertext, b)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return gcm, nil
}

// openWithKey decrypts a prefixed ciphertext using the given raw key.
func openWithKey(ciphertext string, key []byte) (string, error) {
	data := strings.TrimPrefix(ciphertext, EncryptedPrefix)
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return "", ErrInvalidEncryptedFormat
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}
	encryptedData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", ErrInvalidEncryptedFormat
	}
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	return string(plaintext), nil
}

// GetEncryptionStatus returns "encrypted", "plaintext", or "empty" for a value.
func GetEncryptionStatus(value string) string {
	if value == "" {
		return "empty"
	}
	if IsEncrypted(value) {
		return "encrypted"
	}
	return "plaintext"
}
