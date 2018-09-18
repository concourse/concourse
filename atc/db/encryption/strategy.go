package encryption

import "errors"

var ErrDataIsEncrypted = errors.New("failed to decrypt data that is encrypted")
var ErrDataIsNotEncrypted = errors.New("failed to decrypt data that is not encrypted")

//go:generate counterfeiter . Strategy

type Strategy interface {
	Encrypt([]byte) (string, *string, error)
	Decrypt(string, *string) ([]byte, error)
}
