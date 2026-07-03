package cipher_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/govpn/govpn/internal/cipher"
	"github.com/govpn/govpn/pkg/testutil"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	c := mustNew(t, "test-passphrase")
	want := []byte("Hello, VPN tunnel!")

	enc, err := c.Encrypt(want)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("round-trip: got %q, want %q", got, want)
	}
}

func TestEncryptNonDeterministic(t *testing.T) {
	t.Parallel()

	c := mustNew(t, "passphrase")
	pt := []byte("same plaintext every time")

	enc1, _ := c.Encrypt(pt)
	enc2, _ := c.Encrypt(pt)

	if bytes.Equal(enc1, enc2) {
		t.Error("two Encrypt calls produced identical output — possible nonce reuse")
	}
}

func TestDecryptTamperedTag(t *testing.T) {
	t.Parallel()

	c := mustNew(t, "passphrase")
	enc, _ := c.Encrypt([]byte("sensitive data"))

	enc[len(enc)-1] ^= 0xFF // corrupt the last byte of the GCM tag

	_, err := c.Decrypt(enc)
	if !errors.Is(err, cipher.ErrDecryptFailed) {
		t.Errorf("expected ErrDecryptFailed, got %v", err)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	t.Parallel()

	sender := mustNew(t, "sender-key")
	receiver := mustNew(t, "wrong-key")

	enc, _ := sender.Encrypt([]byte("secret"))

	_, err := receiver.Decrypt(enc)
	if !errors.Is(err, cipher.ErrDecryptFailed) {
		t.Errorf("expected ErrDecryptFailed, got %v", err)
	}
}

func TestDecryptTooShort(t *testing.T) {
	t.Parallel()

	c := mustNew(t, "passphrase")

	_, err := c.Decrypt([]byte("short"))
	if !errors.Is(err, cipher.ErrMessageTooShort) {
		t.Errorf("expected ErrMessageTooShort, got %v", err)
	}
}

func TestNewBadKeySize(t *testing.T) {
	t.Parallel()

	_, err := cipher.New([]byte("tooshort"))
	if !errors.Is(err, cipher.ErrInvalidKeySize) {
		t.Errorf("expected ErrInvalidKeySize, got %v", err)
	}
}

func TestRoundTripMaxMTU(t *testing.T) {
	t.Parallel()

	c := mustNew(t, "passphrase")
	want := make([]byte, 1400)
	for i := range want {
		want[i] = byte(i)
	}

	enc, err := c.Encrypt(want)
	if err != nil {
		t.Fatalf("Encrypt 1400-byte packet: %v", err)
	}

	got, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt 1400-byte packet: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Error("max-MTU round-trip mismatch")
	}
}

func BenchmarkEncrypt1400(b *testing.B) {
	c := mustNewB(b, "bench-passphrase")
	pt := make([]byte, 1400)
	b.SetBytes(1400)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := c.Encrypt(pt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecrypt1400(b *testing.B) {
	c := mustNewB(b, "bench-passphrase")
	pt := make([]byte, 1400)
	enc, _ := c.Encrypt(pt)
	b.SetBytes(1400)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := c.Decrypt(enc); err != nil {
			b.Fatal(err)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustNew(t *testing.T, pass string) *cipher.AEAD {
	return testutil.MustNew(t, pass)
}

func mustNewB(b *testing.B, pass string) *cipher.AEAD {
	return testutil.MustNewB(b, pass)
}
