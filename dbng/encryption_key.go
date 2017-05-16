package dbng

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
)

type EncryptionKey struct {
	block cipher.Block
}

func NewEncryptionKey(b cipher.Block) *EncryptionKey {
	return &EncryptionKey{
		block: b,
	}
}

func (e EncryptionKey) Encrypt(plaintext []byte) (string, string, error) {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}

	aesgcm, err := cipher.NewGCM(e.block)
	if err != nil {
		return "", "", err
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	return hex.EncodeToString(ciphertext), hex.EncodeToString(nonce), nil
}

func (e EncryptionKey) Decrypt(text string, n string) ([]byte, error) {
	if n == "" {
		return []byte(text), nil
	}

	ciphertext, err := hex.DecodeString(text)
	if err != nil {
		return nil, err
	}

	nonce, err := hex.DecodeString(n)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(e.block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
