package idtoken

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
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

	existingKeys, err := dbSigningKeyFactory.GetAllKeys()
	if err != nil {
		panic(err)
	}
	if len(existingKeys) == 0 {
		logger.Info("Generating signing key for idtoken credential provider")
		// currently only RSA keys are supported
		newKey, err := GenerateNewRSAKey()
		if err != nil {
			return err
		}
		err = dbSigningKeyFactory.CreateKey(*newKey)
		if err != nil {
			return err
		}
	} else {
		logger.Info("Reusing existing signing key for idtoken credential provider")
	}
	return nil
}
