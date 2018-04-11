package dexserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/concourse/skymarshal/bindata"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/coreos/dex/connector/cf"
	"github.com/coreos/dex/connector/github"
	"github.com/coreos/dex/server"
	"github.com/coreos/dex/storage"
	"github.com/coreos/dex/storage/memory"
	"github.com/elazarl/go-bindata-assetfs"
	"golang.org/x/crypto/bcrypt"
)

type DexConfig struct {
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
			Name: "Username",
		})
	}

	if conf, err := newGithubConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "github",
			Type:   "github",
			Name:   "Github",
			Config: conf,
		})
	}

	if conf, err := newCFConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "cf",
			Type:   "cf",
			Name:   "CF",
			Config: conf,
		})
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
		Issuer: "concourse",
		Dir:    assets,
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

func newGithubConfig(config *DexConfig) ([]byte, error) {
	if config.Flags.Github.ClientID != "" && config.Flags.Github.ClientSecret != "" {
		return json.Marshal(github.Config{
			ClientID:     config.Flags.Github.ClientID,
			ClientSecret: config.Flags.Github.ClientSecret,
			RedirectURI:  strings.TrimRight(config.IssuerURL, "/") + "/callback",
		})
	} else {
		return nil, errors.New("Not configured")
	}
}

func newCFConfig(config *DexConfig) ([]byte, error) {
	if config.Flags.CF.ClientID != "" && config.Flags.CF.ClientSecret != "" && config.Flags.CF.APIURL != "" {
		return json.Marshal(cf.Config{
			ClientID:           config.Flags.CF.ClientID,
			ClientSecret:       config.Flags.CF.ClientSecret,
			APIURL:             config.Flags.CF.APIURL,
			RootCAs:            config.Flags.CF.RootCAs,
			InsecureSkipVerify: config.Flags.CF.InsecureSkipVerify,
			RedirectURI:        strings.TrimRight(config.IssuerURL, "/") + "/callback",
		})
	} else {
		return nil, errors.New("Not configured")
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
