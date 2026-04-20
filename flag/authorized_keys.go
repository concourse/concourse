package flag

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

type AuthorizedKeys struct {
	File string
	Keys []ssh.PublicKey
}

func (f *AuthorizedKeys) UnmarshalFlag(value string) error {
	authorizedKeysBytes, err := os.ReadFile(value)
	if err != nil {
		return fmt.Errorf("failed to read authorized keys: %s", err)
	}

	var authorizedKeys []ssh.PublicKey

	for {
		key, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			// there's no good error to check for here
			break
		}

		authorizedKeys = append(authorizedKeys, key)

		authorizedKeysBytes = rest
	}

	f.File = value
	f.Keys = authorizedKeys

	return nil
}

// Reload reloads the value of the Keys
func (f *AuthorizedKeys) Reload() error {
	return f.UnmarshalFlag(f.File)
}
