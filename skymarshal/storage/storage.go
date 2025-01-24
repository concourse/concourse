package storage

import (
	"log/slog"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/dex/storage"
	"github.com/concourse/dex/storage/sql"
	"github.com/concourse/flag/v2"
)

type Storage interface {
	storage.Storage
}

func NewPostgresStorage(log lager.Logger, postgres flag.PostgresConfig) (Storage, error) {
	var host string

	if postgres.Socket != "" {
		host = postgres.Socket
	} else {
		host = postgres.Host
	}

	store := sql.Postgres{
		NetworkDB: sql.NetworkDB{
			Database:          postgres.Database,
			User:              postgres.User,
			Password:          postgres.Password,
			Host:              host,
			Port:              postgres.Port,
			ConnectionTimeout: int(postgres.ConnectTimeout.Seconds()),
		},
		SSL: sql.SSL{
			Mode:     postgres.SSLMode,
			CAFile:   string(postgres.CACert),
			CertFile: string(postgres.ClientCert),
			KeyFile:  string(postgres.ClientKey),
		},
	}

	return store.Open(slog.New(lager.NewHandler(log)))
}
