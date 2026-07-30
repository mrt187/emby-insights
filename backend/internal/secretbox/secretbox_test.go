package secretbox

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	box, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ciphertext, err := box.Encrypt("super-secret-api-key")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if ciphertext == "" || ciphertext == "super-secret-api-key" {
		t.Fatalf("Encrypt() = %q, want an opaque ciphertext", ciphertext)
	}
	plaintext, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "super-secret-api-key" {
		t.Fatalf("Decrypt() = %q, want original plaintext", plaintext)
	}
}

func TestEncryptProducesDifferentCiphertextEachCall(t *testing.T) {
	box, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	first, err := box.Encrypt("same-plaintext")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	second, err := box.Encrypt("same-plaintext")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if first == second {
		t.Fatalf("Encrypt() returned identical ciphertext twice, want a fresh nonce each call")
	}
}

func TestEmptyPlaintextRoundTripsToEmptyCiphertext(t *testing.T) {
	box, err := New(testKey(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ciphertext, err := box.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if ciphertext != "" {
		t.Fatalf("Encrypt(\"\") = %q, want empty string", ciphertext)
	}
	plaintext, err := box.Decrypt("")
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "" {
		t.Fatalf("Decrypt(\"\") = %q, want empty string", plaintext)
	}
}

func TestNewRejectsWrongKeyLength(t *testing.T) {
	shortKey := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := New(shortKey); err == nil {
		t.Fatalf("New() error = nil, want error for wrong key length")
	}
}

func TestNewRejectsInvalidBase64(t *testing.T) {
	if _, err := New("not-valid-base64!!"); err == nil {
		t.Fatalf("New() error = nil, want error for invalid base64")
	}
}
