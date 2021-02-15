package flag

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"

	"github.com/dgrijalva/jwt-go"
)

type PrivateKey struct {
	*rsa.PrivateKey
}

func (p *PrivateKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	return p.Set(value)
}

// Can be removed once flags are deprecated
func (p *PrivateKey) Set(value string) error {
	rsaKeyBlob, err := ioutil.ReadFile(value)
	if err != nil {
		return fmt.Errorf("failed to read private key file (%s): %s", value, err)
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
	if err != nil {
		return err
	}

	p.PrivateKey = key

	return nil
}

// Can be removed once flags are deprecated
func (p *PrivateKey) String() string {
	return string(pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(p.PrivateKey),
		},
	))
}

// Can be removed once flags are deprecated
func (p *PrivateKey) Type() string {
	return "PrivateKey"
}
