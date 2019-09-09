package atccmd

import (
	"context"
	"database/sql"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/encryption"
	"golang.org/x/crypto/acme/autocert"
)

type dbCache struct {
	get, put, delete *sql.Stmt
	es               encryption.Strategy
}

func newDbCache(conn db.Conn) (autocert.Cache, error) {
	c := new(dbCache)
	c.es = conn.EncryptionStrategy()
	var err error
	c.get, err = conn.Prepare("SELECT cert, nonce FROM cert_cache WHERE domain = $1")
	if err != nil {
		return nil, err
	}
	c.put, err = conn.Prepare("INSERT INTO cert_cache (domain, cert, nonce) VALUES ($1, $2, $3) ON CONFLICT (domain) DO UPDATE SET domain = EXCLUDED.domain, cert = EXCLUDED.cert, nonce = EXCLUDED.nonce")
	if err != nil {
		return nil, err
	}
	c.delete, err = conn.Prepare("DELETE FROM cert_cache WHERE domain = $1")
	return c, err
}

func (c *dbCache) Get(ctx context.Context, domain string) ([]byte, error) {
	var ciphertext string
	var nonce sql.NullString
	err := c.get.QueryRowContext(ctx, domain).Scan(&ciphertext, &nonce)
	if err == sql.ErrNoRows {
		err = autocert.ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}
	return c.es.Decrypt(ciphertext, noncense)
}

func (c *dbCache) Put(ctx context.Context, domain string, cert []byte) error {
	ciphertext, nonce, err := c.es.Encrypt(cert)
	if err != nil {
		return err
	}
	_, err = c.put.ExecContext(ctx, domain, ciphertext, nonce)
	return err
}

func (c *dbCache) Delete(ctx context.Context, domain string) error {
	_, err := c.delete.ExecContext(ctx, domain)
	return err
}
