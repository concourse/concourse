package db

import (
	"encoding/json"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-jose/go-jose/v4"
)

type SigningKeyType string

const (
	SigningKeyTypeRSA SigningKeyType = "RSA"
	SigningKeyTypeEC  SigningKeyType = "EC"
)

//counterfeiter:generate . SigningKeyFactory
type SigningKeyFactory interface {
	CreateKey(jwk jose.JSONWebKey) error
	GetAllKeys() ([]SigningKey, error)
	GetNewestKey(keyType SigningKeyType) (SigningKey, error)
}

type signingKeyFactory struct {
	conn DbConn
}

func NewSigningKeyFactory(conn DbConn) SigningKeyFactory {
	return &signingKeyFactory{
		conn: conn,
	}
}

func (f *signingKeyFactory) CreateKey(jwk jose.JSONWebKey) error {
	tx, err := f.conn.Begin()

	if err != nil {
		return err
	}
	defer Rollback(tx)

	encoded, err := json.Marshal(jwk)
	if err != nil {
		return err
	}

	builder := psql.Insert("signing_keys").
		Columns("kid", "kty", "jwk").
		Values(jwk.KeyID, signingKeyTypeFromAlg(jwk.Algorithm), string(encoded))

	_, err = builder.
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *signingKeyFactory) GetAllKeys() ([]SigningKey, error) {
	rows, err := psql.Select("jwk", "created_at").
		From("signing_keys").
		OrderBy("created_at ASC").
		RunWith(f.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	var signingKeys []SigningKey

	for rows.Next() {
		var signingKey signingKey
		var rawJWK []byte
		err = rows.Scan(&rawJWK, &signingKey.createdAt)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawJWK, &signingKey.jwk)
		if err != nil {
			return nil, err
		}

		signingKey.conn = f.conn

		signingKeys = append(signingKeys, signingKey)
	}
	return signingKeys, nil
}

func (f *signingKeyFactory) GetNewestKey(keyType SigningKeyType) (SigningKey, error) {
	rows, err := psql.Select("jwk", "created_at").
		From("signing_keys").
		Where(sq.Eq{"kty": keyType}).
		OrderBy("created_at DESC").
		Limit(1).
		RunWith(f.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	var signingKey signingKey

	if rows.Next() {
		var rawJWK []byte
		err = rows.Scan(&rawJWK, &signingKey.createdAt)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawJWK, &signingKey.jwk)
		if err != nil {
			return nil, err
		}

		signingKey.conn = f.conn

		return signingKey, nil
	}
	return nil, fmt.Errorf("no signing key found with specified type")
}

// the jwk type does not expose the kty header to us, so we have to infer it from the alg
// Key Types: https://datatracker.ietf.org/doc/html/rfc7518#section-6.1
func signingKeyTypeFromAlg(alg string) SigningKeyType {
	keyType := SigningKeyTypeRSA
	if strings.HasPrefix(alg, "ES") {
		keyType = SigningKeyTypeEC
	}
	return keyType
}
