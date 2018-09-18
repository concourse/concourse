package vault

import (
	"context"
	"sync"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
)

type cachedSecret struct {
	deadline time.Time
	secret   *vaultapi.Secret
}

// A Cache caches secrets read from a SecretReader until the lease on
// the secret expires. Once expired the credential is proactively
// deleted from cache to maintain a smaller cache footprint.
type Cache struct {
	sync.RWMutex
	cache    map[string]*cachedSecret
	newItems chan time.Time
	sr       SecretReader
	context  context.Context
	maxLease time.Duration
}

// TODO: Should a cache have a max size to
// prevent unbounded growth?

// NewCache using the underlying vault client.
func NewCache(sr SecretReader, maxLease time.Duration) *Cache {
	c := &Cache{
		cache:    make(map[string]*cachedSecret),
		newItems: make(chan time.Time, 100),
		sr:       sr,
		maxLease: maxLease,
	}
	go c.reaperThread()
	return c
}

func (c *Cache) reapCache() time.Time {
	c.Lock()
	defer c.Unlock()

	var smallestNext time.Time
	for k, secret := range c.cache {
		if time.Now().After(secret.deadline) {
			delete(c.cache, k)
			continue
		}
		if secret.deadline.Before(smallestNext) {
			smallestNext = secret.deadline
		}
	}
	return smallestNext
}

func (c *Cache) reaperThread() {
	sleep := time.NewTimer(1 * time.Second)
	defer sleep.Stop()
	var nextWakeup time.Time
	for {
		select {
		case <-sleep.C:
			nextWakeup = c.reapCache()
			if !nextWakeup.IsZero() {
				sleep.Reset(nextWakeup.Sub(time.Now()))
			}
		case t := <-c.newItems:
			if t.Before(nextWakeup) {
				continue
			}
			nextWakeup = t
			sleep.Reset(t.Sub(time.Now()))
		}
	}
}

// Read a secret from the cache or the underlying client if not
// present.
func (c *Cache) Read(path string) (*vaultapi.Secret, error) {
	// If we have the secret in our cache just return it
	c.RLock() // don't use defer because we want to agressively release this lock
	cs, cached := c.cache[path]
	c.RUnlock()

	if cached && time.Now().Before(cs.deadline) {
		return cs.secret, nil
	}

	// Otherwise fetch the secret using the client. Clients are
	// thread safe for read use.
	secret, err := c.sr.Read(path)
	if err != nil || secret == nil {
		return secret, err
	}

	// We will renew the item in half the lease duration to resolve an inherent race
	// in this setup: What if the secret becomes invalid _during_ the build. We don't
	// want to be issuing secrets that expire in (fex) 100ms.
	// This is a problem in any implementation, as lease duration could be 1s and the
	// build will _probably_ take longer  than that.
	dur := time.Duration(secret.LeaseDuration) * time.Second / 2
	if c.maxLease != 0 && dur > c.maxLease {
		dur = c.maxLease
	}

	// Store the secret in cache
	cs = &cachedSecret{
		deadline: time.Now().Add(dur),
		secret:   secret,
	}
	c.Lock()
	c.cache[path] = cs
	c.Unlock()

	// Tell the reaper thread it has new items to cleanup
	c.newItems <- cs.deadline

	return secret, nil
}
