package server

import (
	"crypto/rsa"
	"github.com/concourse/tsa/tsaflags"
	"github.com/dgrijalva/jwt-go"
	"io/ioutil"
)

type privateKeyConfig struct {
	tsaflags.FileFlag

	PrivateKey *rsa.PrivateKey
}

func (pk *privateKeyConfig) UnmarshalFlag(value string) error {
	err := pk.FileFlag.UnmarshalFlag(value)
	if err != nil {
		return err
	}

	blob, err := ioutil.ReadFile(string(pk.FileFlag))
	if err != nil {
		return err
	}

	pk.PrivateKey, err = jwt.ParseRSAPrivateKeyFromPEM(blob)

	return err
}
