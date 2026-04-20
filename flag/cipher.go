package flag

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Cipher parses the encryption key as raw bytes (original behavior).
type Cipher struct {
	cipher.AEAD
}

func (flag *Cipher) UnmarshalFlag(val string) error {
	return unmarshalCipher(&flag.AEAD, []byte(val))
}

// CipherBase64 parses the encryption key as a base64-encoded string.
type CipherBase64 struct {
	cipher.AEAD
}

func (flag *CipherBase64) UnmarshalFlag(val string) error {
	keyBytes, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return fmt.Errorf("failed to decode base64 encryption key: %s", err)
	}
	return unmarshalCipher(&flag.AEAD, keyBytes)
}

// CipherHex parses the encryption key as a hex-encoded string.
type CipherHex struct {
	cipher.AEAD
}

func (flag *CipherHex) UnmarshalFlag(val string) error {
	keyBytes, err := hex.DecodeString(val)
	if err != nil {
		return fmt.Errorf("failed to decode hex encryption key: %s", err)
	}
	return unmarshalCipher(&flag.AEAD, keyBytes)
}

// unmarshalCipher creates an AES-GCM cipher from the given key bytes.
func unmarshalCipher(aead *cipher.AEAD, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to construct AES cipher: %s", err)
	}

	*aead, err = cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to construct GCM: %s", err)
	}

	return nil
}
