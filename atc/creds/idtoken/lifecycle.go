package idtoken

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/go-jose/go-jose/v3"
)

type SigningKeyLifecycler struct {
	Logger              lager.Logger
	DBSigningKeyFactory db.SigningKeyFactory
	LockFactory         lock.LockFactory

	CheckPeriod       time.Duration
	KeyRotationPeriod time.Duration
	KeyGracePeriod    time.Duration
}

// Run continously performs lifecycle-operations for signing keys. It blocks until ctx is cancelled.
func (l *SigningKeyLifecycler) Run(ctx context.Context) {
	for {
		err := l.RunOnce(ctx)
		if err != nil {
			l.Logger.Error("Error when performing lifecycle operations for signing keys:", err)
		}

		sleepUntil := time.NewTimer(l.CheckPeriod)
		select {
		case <-sleepUntil.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}

func (l *SigningKeyLifecycler) RunOnce(ctx context.Context) error {
	var newLock lock.Lock
	var acquired bool
	var err error
	for {
		// acquire a lock to make sure multiple atc-instances don't generate one new key each
		newLock, acquired, err = l.LockFactory.Acquire(l.Logger, lock.NewSigningKeyLifecycleLockID())
		if err != nil {
			return err
		}
		if acquired {
			break
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	defer newLock.Release()

	err = l.ensureUpToDateKeyExists(db.SigningKeyTypeRSA)
	if err != nil {
		return err
	}

	err = l.ensureUpToDateKeyExists(db.SigningKeyTypeEC)
	if err != nil {
		return err
	}

	err = l.removeSupercededKeys(db.SigningKeyTypeRSA)
	if err != nil {
		return err
	}

	err = l.removeSupercededKeys(db.SigningKeyTypeEC)
	if err != nil {
		return err
	}

	return nil
}

// you must have hold SigningKeyLifecycleLock before using this!
func (l *SigningKeyLifecycler) ensureUpToDateKeyExists(kty db.SigningKeyType) error {
	existingKey, err := l.DBSigningKeyFactory.GetNewestKey(kty)
	if err == nil {
		if l.KeyRotationPeriod != 0 && existingKey.CreatedAt().Add(l.KeyRotationPeriod).Before(time.Now()) {
			l.Logger.Info(fmt.Sprintf("%s signing key %s for idtoken credential provider is too old. Generating new key", kty, existingKey.ID()))
		} else {
			// reuse existing key
			return nil
		}
	} else {
		l.Logger.Info(fmt.Sprintf("Could not find a suitable existing %s signing key for idtoken provider. Generating new key.", kty))
	}

	newKey, err := GenerateNewKey(kty)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	return l.DBSigningKeyFactory.CreateKey(*newKey)
}

func (l *SigningKeyLifecycler) removeSupercededKeys(kty db.SigningKeyType) error {
	newestKey, err := l.DBSigningKeyFactory.GetNewestKey(kty)
	if err != nil {
		return nil
	}

	if time.Now().Before(newestKey.CreatedAt().Add(l.KeyGracePeriod)) {
		// the current newest key is not yet KeyGracePeriod old. Keep the previous keys.
		return nil
	}

	allKeys, err := l.DBSigningKeyFactory.GetAllKeys()
	if err != nil {
		return nil
	}

	for _, key := range allKeys {
		if key.KeyType() == kty && key.ID() != newestKey.ID() {
			l.Logger.Info(fmt.Sprintf("Deleting superceded signing key %s for idtoken provider.", key.ID()))
			err := key.Delete()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// made this way so tests can override key-generation
var GenerateNewKey = generateNewKey

func generateNewKey(kty db.SigningKeyType) (*jose.JSONWebKey, error) {
	switch kty {
	case db.SigningKeyTypeRSA:
		return generateNewRSAKey()
	case db.SigningKeyTypeEC:
		return generateNewECDSAKey()
	}
	return nil, fmt.Errorf("unknown key type %s", kty)
}

func generateNewRSAKey() (*jose.JSONWebKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	return &jose.JSONWebKey{
		KeyID:     generateRandomNumericString(),
		Algorithm: "RS256",
		Key:       privateKey,
		Use:       "sign",
	}, nil
}

func generateNewECDSAKey() (*jose.JSONWebKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return &jose.JSONWebKey{
		KeyID:     generateRandomNumericString(),
		Algorithm: "ES256",
		Key:       privateKey,
		Use:       "sign",
	}, nil
}
