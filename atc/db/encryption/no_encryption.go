package encryption

type NoEncryption struct{}

func NewNoEncryption() *NoEncryption {
	return &NoEncryption{}
}

func (n NoEncryption) Encrypt(plaintext []byte) (string, *string, error) {
	return string(plaintext), nil, nil
}

func (n NoEncryption) Decrypt(text string, nonce *string) ([]byte, error) {
	if nonce != nil {
		return nil, ErrDataIsEncrypted
	}

	return []byte(text), nil
}
