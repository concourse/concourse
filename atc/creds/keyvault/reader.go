package keyvault

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
)

// SecretReader is an interface that can Read a secret from a store. This is
// needed here because there are no fakes provided with the Azure SDK for Go
type SecretReader interface {
	Read(string) (string, error)
}

type keyVaultReader struct {
	client      keyvault.BaseClient
	keyVaultURL string
}

// NewKeyVaultReader returns a keyvault implementation of a SecretReader. It
// requires a fully configured Key Vault client and the URL of the Key Vault
func NewKeyVaultReader(client keyvault.BaseClient, keyVaultURL string) SecretReader {
	return &keyVaultReader{
		client:      client,
		keyVaultURL: keyVaultURL,
	}
}

// Read returns the value of the given secret name or an error if there was an issue fetching the secret
func (k *keyVaultReader) Read(name string) (string, error) {
	// Right now we aren't timing out, we may want to add a configurable option for it later and add it to the context
	val, err := k.client.GetSecret(context.Background(), k.keyVaultURL, name, "")
	if err != nil {
		return "", fmt.Errorf("error while trying to fetch secret from key vault: %s", err)
	}

	return *val.Value, nil
}

// fakeReader is a SecretReader implementation for testing. If error is set, it
// will be returned, otherwise the arbitrary value will be returned
type fakeReader struct {
	Value string
	Err   error
}

func (f *fakeReader) Read(_ string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.Value, nil
}
