package db

import (
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . AccessTokenFactory

type AccessTokenFactory interface {
	CreateAccessToken(token string, claims Claims) error
	GetAccessToken(token string) (AccessToken, bool, error)
}

func NewAccessTokenFactory(conn Conn) AccessTokenFactory {
	return &accessTokenFactory{conn}
}

type accessTokenFactory struct {
	conn Conn
}

func (a *accessTokenFactory) CreateAccessToken(token string, claims Claims) error {
	_, err := psql.Insert("access_tokens").
		Columns("token", "sub", "expires_at", "claims").
		Values(token, claims.Sub, time.Unix(claims.ExpiresAt, 0), claims).
		RunWith(a.conn).
		Exec()
	if err != nil {
		return err
	}
	return nil
}

func (a *accessTokenFactory) GetAccessToken(token string) (AccessToken, bool, error) {
	row := psql.Select("token", "claims").
		From("access_tokens").
		Where(sq.Eq{"token": token}).
		RunWith(a.conn).
		QueryRow()

	var accessToken accessToken
	err := scanAccessToken(&accessToken, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return accessToken, true, nil
}
