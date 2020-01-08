package dexserver

import (
	"context"
	"crypto/rsa"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/skymarshal/logger"
	"github.com/concourse/concourse/skymarshal/skycmd"
	s "github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/dex/server"
	"github.com/concourse/dex/storage"
	"github.com/gobuffalo/packr"
	"golang.org/x/crypto/bcrypt"
)

type DexConfig struct {
	Logger      lager.Logger
	IssuerURL   string
	WebHostURL  string
	SigningKey  *rsa.PrivateKey
	Expiration  time.Duration
	Clients     map[string]string
	Users       map[string]string
	RedirectURL string
	Storage     s.Storage
}

func NewDexServer(config *DexConfig) (*server.Server, error) {

	newDexServerConfig, err := NewDexServerConfig(config)
	if err != nil {
		return nil, err
	}

	return server.NewServerWithKey(context.Background(), newDexServerConfig, config.SigningKey)
}

func NewDexServerConfig(config *DexConfig) (server.Config, error) {

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

	for clientId, clientSecret := range config.Clients {
		clients = append(clients, storage.Client{
			ID:           clientId,
			Secret:       clientSecret,
			RedirectURIs: []string{config.RedirectURL},
		})
	}

	if err := replacePasswords(config.Storage, passwords); err != nil {
		return server.Config{}, err
	}

	if err := replaceClients(config.Storage, clients); err != nil {
		return server.Config{}, err
	}

	if err := replaceConnectors(config.Storage, connectors); err != nil {
		return server.Config{}, err
	}

	webConfig := server.WebConfig{
		LogoURL: strings.TrimRight(config.WebHostURL, "/") + "/themes/concourse/logo.svg",
		HostURL: config.WebHostURL,
		Theme:   "concourse",
		Issuer:  "Concourse",
		Dir:     packr.NewBox("../web"),
	}

	return server.Config{
		PasswordConnector:      "local",
		SupportedResponseTypes: []string{"code", "token", "id_token"},
		SkipApprovalScreen:     true,
		IDTokensValidFor:       config.Expiration,
		Issuer:                 config.IssuerURL,
		Storage:                config.Storage,
		Web:                    webConfig,
		Logger:                 logger.New(config.Logger),
	}, nil
}

func replacePasswords(store s.Storage, passwords []storage.Password) error {
	existing, err := store.ListPasswords()
	if err != nil {
		return err
	}

	for _, oldPass := range existing {
		err = store.DeletePassword(oldPass.Email)
		if err != nil {
			return err
		}
	}

	for _, newPass := range passwords {
		err = store.CreatePassword(newPass)
		//if this already exists, some other ATC process has created it already
		//we can assume that both ATCs have the same desired config.
		if err != nil && err != storage.ErrAlreadyExists {
			return err
		}
	}

	return nil
}

func replaceClients(store s.Storage, clients []storage.Client) error {
	existing, err := store.ListClients()
	if err != nil {
		return err
	}

	for _, oldClient := range existing {
		err = store.DeleteClient(oldClient.ID)
		if err != nil {
			return err
		}
	}

	for _, newClient := range clients {
		err = store.CreateClient(newClient)
		//if this already exists, some other ATC process has created it already
		//we can assume that both ATCs have the same desired config.
		if err != nil && err != storage.ErrAlreadyExists {
			return err
		}
	}

	return nil
}

func replaceConnectors(store s.Storage, connectors []storage.Connector) error {
	existing, err := store.ListConnectors()
	if err != nil {
		return err
	}

	for _, oldConn := range existing {
		err = store.DeleteConnector(oldConn.ID)
		if err != nil {
			return err
		}
	}

	for _, newConn := range connectors {
		err = store.CreateConnector(newConn)
		//if this already exists, some other ATC process has created it already
		//we can assume that both ATCs have the same desired config.
		if err != nil && err != storage.ErrAlreadyExists {
			return err
		}
	}

	return nil
}

func newLocalUsers(config *DexConfig) map[string][]byte {
	users := map[string][]byte{}

	for username, password := range config.Users {
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
