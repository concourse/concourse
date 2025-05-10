package idtoken

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/go-jose/go-jose/v3"
)

func EnsureSigningKeysExist(logger lager.Logger, dbSigningKeyFactory db.SigningKeyFactory, logFactory lock.LockFactory) error {
	var newLock lock.Lock
	var acquired bool
	var err error
	for {
		// acquire a lock to make sure multiple atc-instances don't generate one new key each
		newLock, acquired, err = logFactory.Acquire(logger, lock.NewSigningKeyLifecycleLockID())
		if err != nil {
			return err
		}
		if acquired {
			break
		}
	}
	defer newLock.Release()

	err = ensureKeyOfTypeExists(logger, dbSigningKeyFactory, db.SigningKeyTypeRSA)
	if err != nil {
		return err
	}

	err = ensureKeyOfTypeExists(logger, dbSigningKeyFactory, db.SigningKeyTypeEC)
	if err != nil {
		return err
	}
	return nil
}

// you must have hold SigningKeyLifecycleLock before using this!
func ensureKeyOfTypeExists(logger lager.Logger, dbSigningKeyFactory db.SigningKeyFactory, kty db.SigningKeyType) error {
	_, err := dbSigningKeyFactory.GetNewestKey(kty)
	if err == nil {
		logger.Info(fmt.Sprintf("Reusing existing %s signing key for idtoken credential provider", kty))
		return nil
	}

	logger.Info(fmt.Sprintf("Could not find an existing %s signing key for idtoken provider. Generating new one.", kty))
	var newKey *jose.JSONWebKey
	switch kty {
	case db.SigningKeyTypeRSA:
		newKey, err = GenerateNewRSAKey()
	case db.SigningKeyTypeEC:
		newKey, err = GenerateNewECDSAKey()
	default:
		return fmt.Errorf("unknown key type %s", kty)
	}

	if err != nil {
		return err
	}

	return dbSigningKeyFactory.CreateKey(*newKey)

}

func GenerateNewRSAKey() (*jose.JSONWebKey, error) {
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

func GenerateNewECDSAKey() (*jose.JSONWebKey, error) {
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
