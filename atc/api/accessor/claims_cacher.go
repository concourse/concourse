package accessor

import (
	"sync"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc/db"
	"github.com/golang/groupcache/lru"
)

type claimsCacheEntry struct {
	claims db.Claims
	size   int
}

type claimsCacher struct {
	accessTokenFetcher AccessTokenFetcher
	maxCacheSizeBytes  int

	cache          *lru.Cache
	cacheSizeBytes int
	mu             sync.Mutex // lru.Cache is not safe for concurrent access
}

func NewClaimsCacher(
	accessTokenFetcher AccessTokenFetcher,
	maxCacheSizeBytes int,
) *claimsCacher {
	c := &claimsCacher{
		accessTokenFetcher: accessTokenFetcher,
		maxCacheSizeBytes:  maxCacheSizeBytes,
		cache:              lru.New(0),
	}
	c.cache.OnEvicted = func(_ lru.Key, value any) {
		entry, _ := value.(claimsCacheEntry)
		c.cacheSizeBytes -= entry.size
	}

	return c
}

func (c *claimsCacher) GetAccessToken(rawToken string) (db.AccessToken, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	claims, found := c.cache.Get(rawToken)
	if found {
		entry, _ := claims.(claimsCacheEntry)
		return db.AccessToken{Token: rawToken, Claims: entry.claims}, true, nil
	}

	token, found, err := c.accessTokenFetcher.GetAccessToken(rawToken)
	if err != nil {
		return db.AccessToken{}, false, err
	}
	if !found {
		return db.AccessToken{}, false, nil
	}
	payload, err := sonic.Marshal(token.Claims)
	if err != nil {
		return db.AccessToken{}, false, err
	}
	entry := claimsCacheEntry{claims: token.Claims, size: len(payload)}
	c.cache.Add(rawToken, entry)
	c.cacheSizeBytes += entry.size

	for c.cacheSizeBytes > c.maxCacheSizeBytes && c.cache.Len() > 0 {
		c.cache.RemoveOldest()
	}

	return token, true, nil
}

func (c *claimsCacher) DeleteAccessToken(rawToken string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Remove(rawToken)

	return nil
}
