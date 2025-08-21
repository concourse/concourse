package db

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-jose/go-jose/v4"
)

type signingKey struct {
	jwk       jose.JSONWebKey
	createdAt time.Time
	conn      DbConn
}

//counterfeiter:generate . SigningKey
type SigningKey interface {
	ID() string
	KeyType() SigningKeyType
	JWK() jose.JSONWebKey
	CreatedAt() time.Time
	Delete() error
}

func (s signingKey) ID() string {
	return s.jwk.KeyID
}

func (s signingKey) KeyType() SigningKeyType {
	return signingKeyTypeFromAlg(s.jwk.Algorithm)
}

func (s signingKey) JWK() jose.JSONWebKey {
	return s.jwk
}

func (s signingKey) CreatedAt() time.Time {
	return s.createdAt
}

func (s signingKey) Delete() error {
	_, err := psql.Delete("signing_keys").
		Where(sq.Eq{
			"kid": s.ID(),
		}).
		RunWith(s.conn).
		Exec()

	return err
}
