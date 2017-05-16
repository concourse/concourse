package dbng

type NoEncryption struct{}

func NewNoEncryption() *NoEncryption {
	return &NoEncryption{}
}

func (n NoEncryption) Encrypt(plaintext []byte) (string, string, error) {
	return string(plaintext), "", nil
}

func (n NoEncryption) Decrypt(text string, nonce string) ([]byte, error) {
	return []byte(text), nil
}
