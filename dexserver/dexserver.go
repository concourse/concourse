package dexserver

import (
	"context"
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/bindata"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/concourse/dex/server"
	"github.com/concourse/dex/storage"
	"github.com/concourse/dex/storage/memory"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type DexConfig struct {
	Logger       lager.Logger
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Flags        skycmd.AuthFlags
}

func NewDexServer(config *DexConfig) (*server.Server, error) {
	return server.NewServer(context.Background(), NewDexServerConfig(config))
}

func NewDexServerConfig(config *DexConfig) server.Config {
	var clients []storage.Client
	var connectors []storage.Connector
	var passwords []storage.Password

	for username, password := range newLocalUsers(config) {
		passwords = append(passwords, storage.Password{
			UserID:   username,
			Username: username,
			Email:    username,
			Hash:     password,
		})
	}

	if len(passwords) > 0 {
		connectors = append(connectors, storage.Connector{
			ID:   "local",
			Type: "local",
			Name: "Username/Password",
		})
	}

	redirectURI := strings.TrimRight(config.IssuerURL, "/") + "/callback"

	for _, connector := range skycmd.GetConnectors() {
		if c, err := connector.Serialize(redirectURI); err == nil {
			connectors = append(connectors, storage.Connector{
				ID:     connector.ID(),
				Type:   connector.ID(),
				Name:   connector.Name(),
				Config: c,
			})
		}
	}

	clients = append(clients, storage.Client{
		ID:           config.ClientID,
		Secret:       config.ClientSecret,
		RedirectURIs: []string{config.RedirectURL},
	})

	store := memory.New(nil)
	store = storage.WithStaticClients(store, clients)
	store = storage.WithStaticConnectors(store, connectors)
	store = storage.WithStaticPasswords(store, passwords, nil)

	assets := &assetfs.AssetFS{
		Asset:     bindata.Asset,
		AssetDir:  bindata.AssetDir,
		AssetInfo: bindata.AssetInfo,
	}

	webConfig := server.WebConfig{
		LogoURL: strings.TrimRight(config.IssuerURL, "/") + "/themes/concourse/logo.svg",
		Theme:   "concourse",
		Issuer:  "Concourse",
		Dir:     assets,
	}

	var log = &logrus.Logger{
		Out:       ioutil.Discard,
		Hooks:     make(logrus.LevelHooks),
		Formatter: new(logrus.JSONFormatter),
		Level:     logrus.DebugLevel,
	}

	log.Hooks.Add(NewLagerHook(config.Logger))

	return server.Config{
		PasswordConnector:      "local",
		SupportedResponseTypes: []string{"code", "token", "id_token"},
		SkipApprovalScreen:     true,
		Issuer:                 config.IssuerURL,
		Storage:                store,
		Web:                    webConfig,
		Logger:                 log,
	}
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
