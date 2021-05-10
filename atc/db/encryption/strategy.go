package encryption

import "errors"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

var ErrDataIsEncrypted = errors.New("failed to decrypt data that is encrypted")
var ErrDataIsNotEncrypted = errors.New("failed to decrypt data that is not encrypted")

//counterfeiter:generate . Strategy
type Strategy interface {
	Encrypt([]byte) (string, *string, error)
	Decrypt(string, *string) ([]byte, error)
}
