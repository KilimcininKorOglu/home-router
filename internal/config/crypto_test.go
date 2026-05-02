package config_test

import (
	"bytes"
	"testing"

	"github.com/KilimcininKorOglu/home-router/internal/config"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := config.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d, want 32", len(key))
	}

	plaintext := []byte("SuperSecret_PPPoE_Password123!")

	ciphertext, err := config.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := config.Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1, _ := config.GenerateKey()
	key2, _ := config.GenerateKey()

	plaintext := []byte("test data")
	ciphertext, _ := config.Encrypt(plaintext, key1)

	_, err := config.Decrypt(ciphertext, key2)
	if err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func TestDecryptTruncated(t *testing.T) {
	_, err := config.Decrypt([]byte("short"), make([]byte, 32))
	if err == nil {
		t.Fatal("decrypt truncated ciphertext should fail")
	}
}
