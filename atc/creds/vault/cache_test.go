package vault

import (
	"testing"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
)

type MockSecretReader struct {
	secrets []*vaultapi.Secret
	reads   []string
}

func (msr *MockSecretReader) Read(path string) (*vaultapi.Secret, error) {
	msr.reads = append(msr.reads, path)
	s := msr.secrets[0]
	msr.secrets = msr.secrets[1:]
	return s, nil
}

func TestCache(t *testing.T) {
	secrets := []*vaultapi.Secret{
		&vaultapi.Secret{
			RequestID:     "1",
			LeaseDuration: 1,
		},
		&vaultapi.Secret{
			RequestID:     "1",
			LeaseDuration: 2,
		},
		&vaultapi.Secret{
			RequestID:     "1",
			LeaseDuration: 10,
		},
	}
	msr := &MockSecretReader{
		secrets: secrets,
	}

	cache := NewCache(msr, 5*time.Second)
	// miss
	secret, err := cache.Read("path1")
	if err != nil {
		t.Error("got error reading valid secret", err)
	}
	if secret.RequestID != secrets[0].RequestID {
		t.Errorf("read secret %s expected %s", secret.RequestID, secrets[0].RequestID)
	}
	if len(msr.reads) != 1 && msr.reads[0] != "path1" {
		t.Errorf("Got reads [%v], expected [\"%s\"]", msr.reads, "path1")
	}

	// hit
	secret, err = cache.Read("path1")
	if err != nil {
		t.Error("got error reading valid secret from cache", err)
	}
	if secret.RequestID != secrets[0].RequestID {
		t.Errorf("read secret %s expected %s", secret.RequestID, secrets[0].RequestID)
	}
	if len(msr.reads) != 1 && msr.reads[0] != "path1" {
		t.Errorf("Got reads [%v], expected [\"%s\"]", msr.reads, "path1")
	}

	// reap
	time.Sleep(time.Duration(secret.LeaseDuration)*time.Second + 100*time.Millisecond)

	// miss
	secret, err = cache.Read("path1")
	if err != nil {
		t.Error("got error reading valid secret from cache", err)
	}
	if secret.RequestID != secrets[0].RequestID {
		t.Errorf("read secret %s expected %s", secret.RequestID, secrets[0].RequestID)
	}
	if len(msr.reads) != 2 && msr.reads[0] != "path1" {
		t.Errorf("Got reads [%v], expected [\"%s\"]", msr.reads, "path1 path1")
	}

	// reap
	time.Sleep(time.Duration(secret.LeaseDuration)*time.Second + 100*time.Millisecond)
	cache.RLock()
	if len(cache.cache) != 0 {
		t.Errorf("Expectde cache to be clean after expiration, was %v", cache.cache)
	}
	cache.RUnlock()

	// Test max duration
	secret, err = cache.Read("path1")
	if err != nil {
		t.Error("got error reading valid secret from cache", err)
	}
	if secret.RequestID != secrets[0].RequestID {
		t.Errorf("read secret %s expected %s", secret.RequestID, secrets[0].RequestID)
	}
	if len(msr.reads) != 1 && msr.reads[0] != "path1" {
		t.Errorf("Got reads [%v], expected [\"%s\"]", msr.reads, "path1")
	}

	// reap
	time.Sleep(5*time.Second + 100*time.Millisecond)
	cache.RLock()
	if len(cache.cache) != 0 {
		t.Errorf("Expectde cache to be clean after maxu duration, was %v", cache.cache)
	}
	cache.RUnlock()

}
