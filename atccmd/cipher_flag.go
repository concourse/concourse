package atccmd

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

type CipherFlag struct {
	cipher.AEAD
}

func (flag *CipherFlag) UnmarshalFlag(val string) error {
	block, err := aes.NewCipher([]byte(val))
	if err != nil {
		return fmt.Errorf("failed to construct AES cipher: %s", err)
	}

	flag.AEAD, err = cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to construct GCM: %s", err)
	}

	return nil
}
