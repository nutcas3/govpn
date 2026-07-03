// Package cipher provides authenticated symmetric encryption for VPN packets.
//
// Every encrypted message has the layout:
//
//	+------------+--------------------+----------+
//	| nonce (12) | ciphertext (var.)  | tag (16) |
//	+------------+--------------------+----------+
//
// AES-256-GCM gives both confidentiality and integrity authentication.
// On amd64/arm64 the Go runtime uses hardware AES instructions automatically.
package cipher

import (
	"crypto/aes"
	gocipher "crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// Fixed sizes for the AEAD construction.
const (
	KeySize   = 32 // AES-256
	NonceSize = 12 // GCM standard nonce
	Overhead  = 16 // GCM authentication tag
	MinMsgLen = NonceSize + Overhead
)

// Sentinel errors allow callers to use errors.Is for targeted handling.
var (
	ErrMessageTooShort = errors.New("cipher: message too short")
	ErrDecryptFailed   = errors.New("cipher: authentication or decryption failed")
	ErrInvalidKeySize  = errors.New("cipher: key must be 32 bytes")
)

// AEAD wraps an AES-256-GCM cipher and exposes Encrypt/Decrypt.
// The zero value is not usable; construct via New or NewFromPassphrase.
type AEAD struct {
	aead gocipher.AEAD
	rand io.Reader // injectable for tests; defaults to crypto/rand.Reader
}

// New constructs an AEAD from a 32-byte key.
func New(key []byte) (*AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidKeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: aes init: %w", err)
	}

	gcm, err := gocipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher: gcm init: %w", err)
	}

	return &AEAD{aead: gcm, rand: rand.Reader}, nil
}

// NewFromPassphrase derives a key from passphrase via SHA-256 and calls New.
//
// NOTE: SHA-256 is a fast hash. For production use, replace with Argon2id
// (golang.org/x/crypto/argon2) with a stored per-peer salt.
func NewFromPassphrase(passphrase string) (*AEAD, error) {
	sum := sha256.Sum256([]byte(passphrase))
	return New(sum[:])
}

// Encrypt encrypts plaintext and returns [nonce || ciphertext || tag].
// A fresh random nonce is generated for each call.
func (a *AEAD) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(a.rand, nonce); err != nil {
		return nil, fmt.Errorf("cipher: generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to the nonce slice in one allocation.
	return a.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt verifies and decrypts a message produced by Encrypt.
func (a *AEAD) Decrypt(msg []byte) ([]byte, error) {
	if len(msg) < MinMsgLen {
		return nil, ErrMessageTooShort
	}

	nonce, ciphertext := msg[:NonceSize], msg[NonceSize:]

	plaintext, err := a.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Wrap the sentinel so callers can errors.Is(err, ErrDecryptFailed).
		return nil, fmt.Errorf("%w: %w", ErrDecryptFailed, err)
	}

	return plaintext, nil
}
