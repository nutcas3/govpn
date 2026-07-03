package testutil

import (
	"testing"

	"github.com/govpn/govpn/internal/cipher"
)

func MustNew(t *testing.T, pass string) *cipher.AEAD {
	t.Helper()
	c, err := cipher.NewFromPassphrase(pass)
	if err != nil {
		t.Fatalf("NewFromPassphrase: %v", err)
	}
	return c
}

func MustNewB(b *testing.B, pass string) *cipher.AEAD {
	b.Helper()
	c, err := cipher.NewFromPassphrase(pass)
	if err != nil {
		b.Fatalf("NewFromPassphrase: %v", err)
	}
	return c
}
