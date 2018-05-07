package dexserver

import (
	"context"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/bindata"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/coreos/dex/server"
	"github.com/coreos/dex/storage"
	"github.com/coreos/dex/storage/memory"
	"github.com/elazarl/go-bindata-assetfs"
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
		} else {
			config.Logger.Error("connector-config-error", err, lager.Data{
				"connector": connector.Name(),
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

	return server.Config{
		PasswordConnector:      "local",
		SupportedResponseTypes: []string{"code", "token", "id_token"},
		SkipApprovalScreen:     true,
		Issuer:                 config.IssuerURL,
		Storage:                store,
		Web:                    webConfig,
	}
}

func newLocalUsers(config *DexConfig) map[string][]byte {
	users := map[string][]byte{}

	for username, password := range config.Flags.LocalUsers {
		if username != "" && password != "" {
			if encrypted, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err == nil {
				users[username] = encrypted
			}
		}
	}

	return users
}
