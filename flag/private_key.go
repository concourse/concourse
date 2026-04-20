package flag

import (
	"crypto/rsa"
	"fmt"
	"os"

	jwt "github.com/golang-jwt/jwt/v5"
)

type PrivateKey struct {
	*rsa.PrivateKey
}

func (f *PrivateKey) UnmarshalFlag(path string) error {
	rsaKeyBlob, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read private key file (%s): %s", path, err)
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
	if err != nil {
		return err
	}

	f.PrivateKey = key

	return nil
}
