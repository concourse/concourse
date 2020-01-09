package storage

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/skymarshal/logger"
	"github.com/concourse/dex/storage"
	"github.com/concourse/dex/storage/sql"
	"github.com/concourse/flag"
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
		SSL: sql.SSL{
			Mode:     postgres.SSLMode,
			CAFile:   string(postgres.CACert),
			CertFile: string(postgres.ClientCert),
			KeyFile:  string(postgres.ClientKey),
		},
	}

	store.Database = postgres.Database
	store.User = postgres.User
	store.Password = postgres.Password
	store.Host = host
	store.Port = postgres.Port
	store.ConnectionTimeout = int(postgres.ConnectTimeout.Seconds())

	return store.Open(logger.New(log))
}
