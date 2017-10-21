package server

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
)

type privateKeyConfig struct {
	PrivateKey *rsa.PrivateKey
}

func (pk *privateKeyConfig) UnmarshalFlag(encoded string) error {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}

	pk.PrivateKey, err = x509.ParsePKCS1PrivateKey(key)

	return err
}
