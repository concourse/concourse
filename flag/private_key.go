package flag

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"

	"github.com/dgrijalva/jwt-go"
)

type PrivateKey struct {
	*rsa.PrivateKey
	originalKey string
}

func (p PrivateKey) MarshalYAML() (interface{}, error) {
	return p.originalKey, nil
}

func (p *PrivateKey) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	if value != "" {
		return p.Set(value)
	}

	return nil
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

	p.originalKey = value
	p.PrivateKey = key

	return nil
}

// Can be removed once flags are deprecated
func (p *PrivateKey) String() string {
	return p.originalKey
}

// Can be removed once flags are deprecated
func (p *PrivateKey) Type() string {
	return "PrivateKey"
}
