package db

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
)

type EncryptionKey struct {
	aesgcm cipher.AEAD
}

func NewEncryptionKey(a cipher.AEAD) *EncryptionKey {
	return &EncryptionKey{
		aesgcm: a,
	}
}

func (e EncryptionKey) Encrypt(plaintext []byte) (string, *string, error) {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", nil, err
	}

	ciphertext := e.aesgcm.Seal(nil, nonce, plaintext, nil)

	noncense := hex.EncodeToString(nonce)

	return hex.EncodeToString(ciphertext), &noncense, nil
}

func (e EncryptionKey) Decrypt(text string, n *string) ([]byte, error) {
	if n == nil {
		return nil, ErrDataIsNotEncrypted
	}

	ciphertext, err := hex.DecodeString(text)
	if err != nil {
		return nil, err
	}

	nonce, err := hex.DecodeString(*n)
	if err != nil {
		return nil, err
	}

	plaintext, err := e.aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
