package dexserver

import (
	"context"
	"strings"

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
	Logger       lager.Logger
	IssuerURL    string
	WebHostURL   string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Flags        skycmd.AuthFlags
	Storage      s.Storage
}

func NewDexServer(config *DexConfig) (*server.Server, error) {
	newDexServerConfig, err := NewDexServerConfig(config)
	if err != nil {
		return nil, err
	}

	return server.NewServer(context.Background(), newDexServerConfig)
}

func NewDexServerConfig(config *DexConfig) (server.Config, error) {

	log := logger.New(config.Logger)

	localUsersToAdd := newLocalUsers(config)

	store := config.Storage
	storedPasses, err := store.ListPasswords()
	if err != nil {
		return server.Config{}, err
	}

	// First clear out users from dex store that are no longer in params
	for _, pass := range storedPasses {
		if _, exists := localUsersToAdd[pass.Email]; !exists {
			removePasswordFromStore(store, pass.Email)
		}
	}

	// Then add new local users to dex store
	var localAuthConfigured = false
	for username, password := range localUsersToAdd {
		err = createPasswordInStore(store,
			storage.Password{
				UserID:   username,
				Username: username,
				Email:    username,
				Hash:     password,
			},
			true)
		if err != nil {
			return server.Config{}, err
		}

		if !localAuthConfigured {
			err = createConnectorInStore(store,
				storage.Connector{
					ID:   "local",
					Type: "local",
					Name: "Username/Password",
				},
				false)
			if err != nil {
				return server.Config{}, err
			}
			localAuthConfigured = true
		}
	}

	redirectURI := strings.TrimRight(config.IssuerURL, "/") + "/callback"

	for _, connector := range skycmd.GetConnectors() {
		if c, err := connector.Serialize(redirectURI); err == nil {
			err = createConnectorInStore(store,
				storage.Connector{
					ID:     connector.ID(),
					Type:   connector.ID(),
					Name:   connector.Name(),
					Config: c,
				},
				true)
			if err != nil {
				return server.Config{}, err
			}
		} else {
			// connector has not been configured, or has not been configured properly
			err = removeConnectorFromStore(store, connector.ID())
			if err != nil {
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
		Issuer:                 config.IssuerURL,
		Storage:                store,
		Web:                    webConfig,
		Logger:                 log,
	}, nil
}

// Creates a password for the given username in the dex store.  If username and password already exists
// in the store and update is set to true, the dex store will be updated with the password.
func createPasswordInStore(store storage.Storage, password storage.Password, update bool) error {
	existingPass, err := store.GetPassword(password.Email)
	if err == storage.ErrNotFound || existingPass.Email == "" {
		err = store.CreatePassword(password)
		if err != nil {
			return err
		}
	} else if err == nil {
		if update {
			err = store.UpdatePassword(
				password.Email,
				func(_ storage.Password) (storage.Password, error) { return password, nil },
			)
			if err != nil {
				return err
			}
		}
	} else {
		return err
	}

	return nil
}

// Checks if password exists and removes it if it does
func removePasswordFromStore(store storage.Storage, email string) error {
	_, err := store.GetPassword(email)
	if err == nil {
		// password exists, so remove it
		err = store.DeletePassword(email)
		if err != nil {
			return err
		}
	} else if err != storage.ErrNotFound {
		return err
	}

	return nil
}

// Creates a connector in the dex store.  If it already exists in the store and update is set to true,
// the dex store will be updated with the connector.
func createConnectorInStore(store storage.Storage, connector storage.Connector, update bool) error {
	_, err := store.GetConnector(connector.ID)
	if err == storage.ErrNotFound {
		err = store.CreateConnector(connector)
		if err != nil {
			return err
		}
	} else if err == nil {
		if update {
			err = store.UpdateConnector(
				connector.ID,
				func(_ storage.Connector) (storage.Connector, error) { return connector, nil },
			)
			if err != nil {
				return err
			}
		}
	} else {
		return err
	}

	return nil
}

// Checks if connector exists and removes it if it does
func removeConnectorFromStore(store storage.Storage, connectorID string) error {
	_, err := store.GetConnector(connectorID)
	if err == nil {
		// connector exists, so remove it
		err = store.DeleteConnector(connectorID)
		if err != nil {
			return err
		}
	} else if err != storage.ErrNotFound {
		return err
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
