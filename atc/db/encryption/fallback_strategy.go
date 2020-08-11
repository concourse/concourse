package encryption

type FallbackStrategy struct {
	main     Strategy
	fallback Strategy
}

func NewFallbackStrategy(main, fallback Strategy) *FallbackStrategy {
	return &FallbackStrategy{
		main:     main,
		fallback: fallback,
	}
}

func (n *FallbackStrategy) Encrypt(plaintext []byte) (string, *string, error) {
	return n.main.Encrypt(plaintext)
}

func (n *FallbackStrategy) Decrypt(text string, nonce *string) ([]byte, error) {
	plaintext, err := n.main.Decrypt(text, nonce)
	if err == nil {
		return plaintext, nil
	}
	return n.fallback.Decrypt(text, nonce)
}

