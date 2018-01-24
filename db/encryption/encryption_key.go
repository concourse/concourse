package encryption

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
)

type Key struct {
	aesgcm cipher.AEAD
}

func NewKey(a cipher.AEAD) *Key {
	return &Key{
		aesgcm: a,
	}
}

func (e Key) Encrypt(plaintext []byte) (string, *string, error) {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", nil, err
	}

	ciphertext := e.aesgcm.Seal(nil, nonce, plaintext, nil)

	noncense := hex.EncodeToString(nonce)

	return hex.EncodeToString(ciphertext), &noncense, nil
}

func (e Key) Decrypt(text string, n *string) ([]byte, error) {
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
