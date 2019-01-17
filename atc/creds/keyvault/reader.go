package keyvault

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest"
)

// ErrSecretNotFound is the error returned when a secret cannot be found by the
// reader
var ErrSecretNotFound = fmt.Errorf("secret not found")

// SecretReader is an interface that can get and list secrets from a store.
// This is needed here because there are no fakes provided with the Azure SDK
// for Go
type SecretReader interface {
	Get(string) (string, bool, error)
	List(string) ([]string, error)
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

// Get returns the value of the given secret name, a bool if it was found, and
// an error if there was an issue fetching the secret
func (k *keyVaultReader) Get(name string) (string, bool, error) {
	// Right now we aren't timing out, we may want to add a configurable option
	// for it later and add it to the context
	val, err := k.client.GetSecret(context.Background(), k.keyVaultURL, name, "")
	if err != nil {
		if isKeyVault404(err) {
			return "", false, nil
		}
		return "", false, err
	}

	return *val.Value, true, nil
}

// List returns a list of all secret names with the given prefix. Azure keyvault
// doesn't supported nested paths, so everything is top level and has to be
// filtered out by prefix
func (k *keyVaultReader) List(prefix string) ([]string, error) {
	// Right now we aren't timing out, we may want to add a configurable option
	// for it later and add it to the context
	ctx := context.Background()
	iter, err := k.client.GetSecretsComplete(ctx, k.keyVaultURL, nil)
	if err != nil {
		return nil, err
	}

	var results []string
	// For some reason the Azure SDK uses an iterator, so we get this weird
	// piece of logic
	for iter.NotDone() {
		sec := iter.Value()
		if strings.HasPrefix(*sec.ID, prefix) {
			results = append(results, *sec.ID)
		}
		err := iter.NextWithContext(ctx)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func isKeyVault404(err error) bool {
	e, ok := err.(autorest.DetailedError)
	if !ok || e.Response == nil {
		return false
	}
	return e.Response.StatusCode == http.StatusNotFound
}

// FakeReader is a wrapper that implements the SecretReader interface for
// testing. Each function can be set independently depending on the needs of
// the test
type FakeReader struct {
	GetFunc  func(string) (string, bool, error)
	ListFunc func(string) ([]string, error)
}

func (f *FakeReader) Get(name string) (string, bool, error) {
	if f.GetFunc == nil {
		return "", false, nil
	}
	return f.GetFunc(name)
}

func (f *FakeReader) List(prefix string) ([]string, error) {
	if f.ListFunc == nil {
		return nil, nil
	}
	return f.ListFunc(prefix)
}
