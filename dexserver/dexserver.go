package dexserver

import (
	"context"
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/dex/server"
	"github.com/concourse/dex/storage"
	"github.com/concourse/dex/storage/sql"
	"github.com/concourse/flag"
	"github.com/concourse/skymarshal/bindata"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type DexConfig struct {
	Logger       lager.Logger
	IssuerURL    string
	WebHostURL   string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Flags        skycmd.AuthFlags
	Postgres     flag.PostgresConfig
}

func NewDexServer(config *DexConfig) (*server.Server, error) {
	newDexServerConfig, err := NewDexServerConfig(config)
	if err != nil {
		return nil, err
	}

	return server.NewServer(context.Background(), newDexServerConfig)
}

func NewDexServerConfig(config *DexConfig) (server.Config, error) {
	var log = &logrus.Logger{
		Out:       ioutil.Discard,
		Hooks:     make(logrus.LevelHooks),
		Formatter: new(logrus.JSONFormatter),
		Level:     logrus.DebugLevel,
	}

	log.Hooks.Add(NewLagerHook(config.Logger))

	postgres := config.Postgres

	var host string

	if postgres.Socket != "" {
		host = postgres.Socket
	} else {
		host = postgres.Host
	}

	db := sql.Postgres{
		Database: postgres.Database,
		User:     postgres.User,
		Password: postgres.Password,
		Host:     host,
		Port:     postgres.Port,
		SSL: sql.PostgresSSL{
			Mode:     postgres.SSLMode,
			CAFile:   string(postgres.CACert),
			CertFile: string(postgres.ClientCert),
			KeyFile:  string(postgres.ClientKey),
		},
		ConnectionTimeout: int(postgres.ConnectTimeout.Seconds()),
	}

	store, err := db.Open(log)
	if err != nil {
		return server.Config{}, err
	}

	var localAuthConfigured = false
	localUsers := newLocalUsers(config)
	for username, password := range localUsers {
		pass, err := store.GetPassword(username)
		if err == storage.ErrNotFound || pass.Email == "" {
			err = store.CreatePassword(storage.Password{
				UserID:   username,
				Username: username,
				Email:    username,
				Hash:     password,
			})
			if err != nil {
				return server.Config{}, err
			}
		} else if err != nil {
			return server.Config{}, err
		}

		if !localAuthConfigured {
			_, err = store.GetConnector("local")
			if err == storage.ErrNotFound {
				err = store.CreateConnector(storage.Connector{
					ID:   "local",
					Type: "local",
					Name: "Username/Password",
				})
				if err != nil {
					return server.Config{}, err
				}
			} else if err != nil {
				return server.Config{}, err
			}
			localAuthConfigured = true
		}
	}

	redirectURI := strings.TrimRight(config.IssuerURL, "/") + "/callback"

	for _, connector := range skycmd.GetConnectors() {
		if c, err := connector.Serialize(redirectURI); err == nil {
			_, err = store.GetConnector(connector.ID())
			if err == storage.ErrNotFound {
				err = store.CreateConnector(storage.Connector{
					ID:     connector.ID(),
					Type:   connector.ID(),
					Name:   connector.Name(),
					Config: c,
				})
				if err != nil {
					return server.Config{}, err
				}
			} else if err != nil {
				return server.Config{}, err
			}
		}
	}

	client := storage.Client{
		ID:           config.ClientID,
		Secret:       config.ClientSecret,
		RedirectURIs: []string{config.RedirectURL},
	}

	_, err = store.GetClient(config.ClientID)
	if err == storage.ErrNotFound {
		err = store.CreateClient(client)
		if err != nil {
			return server.Config{}, err
		}
	} else if err == nil {
		err = store.UpdateClient(
			config.ClientID,
			func(_ storage.Client) (storage.Client, error) { return client, nil },
		)
		if err != nil {
			return server.Config{}, err
		}
	} else {
		return server.Config{}, err
	}

	assets := &assetfs.AssetFS{
		Asset:     bindata.Asset,
		AssetDir:  bindata.AssetDir,
		AssetInfo: bindata.AssetInfo,
	}

	webConfig := server.WebConfig{
		LogoURL: strings.TrimRight(config.WebHostURL, "/") + "/themes/concourse/logo.svg",
		HostURL: config.WebHostURL,
		Theme:   "concourse",
		Issuer:  "Concourse",
		Dir:     assets,
	}

	return server.Config{
		PasswordConnector:      "local",
		SupportedResponseTypes: []string{"code", "token", "id_token"},
		SkipApprovalScreen:     true,
		Issuer:                 config.IssuerURL,
		Storage:                store,
		Web:                    webConfig,
		Logger:                 log,
	}, nil
}

func NewLagerHook(logger lager.Logger) *lagerHook {
	return &lagerHook{logger}
}

type lagerHook struct {
	lager.Logger
}

func (self *lagerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (self *lagerHook) Fire(entry *logrus.Entry) error {
	switch entry.Level {
	case logrus.DebugLevel:
		self.Logger.Debug("event", lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.InfoLevel:
		self.Logger.Info("event", lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.WarnLevel:
		self.Logger.Info("event", lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.ErrorLevel:
		self.Logger.Error("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.FatalLevel:
		self.Logger.Fatal("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.PanicLevel:
		self.Logger.Fatal("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	}
	return nil
}

func newLocalUsers(config *DexConfig) map[string][]byte {
	users := map[string][]byte{}

	for username, password := range config.Flags.LocalUsers {
		if username != "" && password != "" {

			var hashed []byte

			if _, err := bcrypt.Cost([]byte(password)); err != nil {
				if hashed, err = bcrypt.GenerateFromPassword([]byte(password), 0); err != nil {

					config.Logger.Error("bcrypt-local-user", err, lager.Data{
						"username": username,
					})

					continue
				}
			} else {
				hashed = []byte(password)
			}

			users[username] = hashed
		}
	}

	return users

}
