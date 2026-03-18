package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes
	plaintext := []byte("ssh-rsa AAAA... very secret key content")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted text does not match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := []byte("01234567890123456789012345678901")
	key2 := []byte("99999999999999999999999999999999")
	plaintext := []byte("secret data")

	ciphertext, err := Encrypt(plaintext, key1)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = Decrypt(ciphertext, key2)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestInvalidKeyLength(t *testing.T) {
	short := []byte("tooshort")
	data := []byte("test")

	_, err := Encrypt(data, short)
	if err == nil {
		t.Fatal("expected error for short key")
	}

	_, err = Decrypt(data, short)
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDifferentCiphertexts(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	plaintext := []byte("same data")

	ct1, _ := Encrypt(plaintext, key)
	ct2, _ := Encrypt(plaintext, key)

	if bytes.Equal(ct1, ct2) {
		t.Fatal("two encryptions of same data should produce different ciphertext (different nonce)")
	}
}
