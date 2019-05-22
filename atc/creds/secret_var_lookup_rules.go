package creds

// SecretLookupPath transforms variable name into full secret path
type SecretLookupPath interface {
	VariableToSecretPath(string) (string, error)
}

// SecretLookupWithPrefix is an implementation which returns [prefix][separator][varName]
type SecretLookupWithPrefix struct {
	Prefix string
}

func NewSecretLookupWithPrefix(prefix string) SecretLookupPath {
	return &SecretLookupWithPrefix{
		Prefix: prefix,
	}
}

func (sl SecretLookupWithPrefix) VariableToSecretPath(varName string) (string, error) {
	return sl.Prefix + varName, nil
}
