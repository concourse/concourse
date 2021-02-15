package flag

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

type Cipher struct {
	cipher.AEAD
}

func (c *Cipher) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	return c.Set(value)
}

// Can be removed once flags are deprecated
func (c *Cipher) Set(value string) error {
	block, err := aes.NewCipher([]byte(value))
	if err != nil {
		return fmt.Errorf("failed to construct AES cipher: %s", err)
	}

	c.AEAD, err = cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to construct GCM: %s", err)
	}

	return nil
}

// Can be removed once flags are deprecated
// XXX [cf]: Can this be not returning the cipher back?
func (c *Cipher) String() string {
	return ""
}

// Can be removed once flags are deprecated
func (c *Cipher) Type() string {
	return "AEAD"
}
