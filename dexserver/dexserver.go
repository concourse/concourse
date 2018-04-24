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
	"github.com/coreos/dex/connector/ldap"
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
			Name: "Username/Password",
		})
	}

	if conf, err := newLDAPConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "ldap",
			Type:   "ldap",
			Name:   "LDAP",
			Config: conf,
		})
	}

	if conf, err := newGithubConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "github",
			Type:   "github",
			Name:   "GitHub",
			Config: conf,
		})
	}

	if conf, err := newGitlabConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "gitlab",
			Type:   "gitlab",
			Name:   "GitLab",
			Config: conf,
		})
	}

	if conf, err := newOIDCConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "oidc",
			Type:   "oidc",
			Name:   "OIDC",
			Config: conf,
		})
	}

	if conf, err := newSAMLConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "saml",
			Type:   "saml",
			Name:   "SAML",
			Config: conf,
		})
	}

	if conf, err := newCFConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "cf",
			Type:   "cf",
			Name:   "Cloud Foundry",
			Config: conf,
		})
	}

	if conf, err := newAuthProxyConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "authproxy",
			Type:   "authproxy",
			Name:   "Auth Proxy",
			Config: conf,
		})
	}

	if conf, err := newLinkedInConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "linkedin",
			Type:   "linkedin",
			Name:   "LinkedIn",
			Config: conf,
		})
	}

	if conf, err := newMicrosftConfig(config); err == nil {
		connectors = append(connectors, storage.Connector{
			ID:     "microsoft",
			Type:   "microsoft",
			Name:   "Microsoft",
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

func newCFConfig(config *DexConfig) ([]byte, error) {
	if config.Flags.CF.IsValid() {
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

func newGithubConfig(config *DexConfig) ([]byte, error) {
	if config.Flags.Github.IsValid() {
		return json.Marshal(github.Config{
			ClientID:     config.Flags.Github.ClientID,
			ClientSecret: config.Flags.Github.ClientSecret,
			RedirectURI:  strings.TrimRight(config.IssuerURL, "/") + "/callback",
		})
	} else {
		return nil, errors.New("Not configured")
	}
}

func newLDAPConfig(config *DexConfig) ([]byte, error) {
	if config.Flags.LDAP.IsValid() {
		ldapConfig := ldap.Config{
			Host:               config.Flags.LDAP.Host,
			BindDN:             config.Flags.LDAP.BindDN,
			BindPW:             config.Flags.LDAP.BindPW,
			InsecureNoSSL:      config.Flags.LDAP.InsecureNoSSL,
			InsecureSkipVerify: config.Flags.LDAP.InsecureSkipVerify,
			StartTLS:           config.Flags.LDAP.StartTLS,
			RootCA:             config.Flags.LDAP.RootCA.Path(),
			RootCAData:         nil,
		}
		ldapConfig.UserSearch.BaseDN = config.Flags.LDAP.UserSearch.BaseDN
		ldapConfig.UserSearch.Filter = config.Flags.LDAP.UserSearch.Filter
		ldapConfig.UserSearch.Username = config.Flags.LDAP.UserSearch.Username
		ldapConfig.UserSearch.Scope = config.Flags.LDAP.UserSearch.Scope
		ldapConfig.UserSearch.IDAttr = config.Flags.LDAP.UserSearch.IDAttr
		ldapConfig.UserSearch.EmailAttr = config.Flags.LDAP.UserSearch.EmailAttr
		ldapConfig.UserSearch.NameAttr = config.Flags.LDAP.UserSearch.NameAttr
		ldapConfig.GroupSearch.BaseDN = config.Flags.LDAP.GroupSearch.BaseDN
		ldapConfig.GroupSearch.Filter = config.Flags.LDAP.GroupSearch.Filter
		ldapConfig.GroupSearch.Scope = config.Flags.LDAP.GroupSearch.Scope
		ldapConfig.GroupSearch.UserAttr = config.Flags.LDAP.GroupSearch.UserAttr
		ldapConfig.GroupSearch.GroupAttr = config.Flags.LDAP.GroupSearch.GroupAttr
		ldapConfig.GroupSearch.NameAttr = config.Flags.LDAP.GroupSearch.NameAttr
		return json.Marshal(ldapConfig)
	} else {
		return nil, errors.New("Not configured")
	}
}

func newGitlabConfig(config *DexConfig) ([]byte, error) {
	return nil, errors.New("Not configured")
}

func newOIDCConfig(config *DexConfig) ([]byte, error) {
	return nil, errors.New("Not configured")
}

func newSAMLConfig(config *DexConfig) ([]byte, error) {
	return nil, errors.New("Not configured")
}

func newAuthProxyConfig(config *DexConfig) ([]byte, error) {
	return nil, errors.New("Not configured")
}

func newLinkedInConfig(config *DexConfig) ([]byte, error) {
	return nil, errors.New("Not configured")
}

func newMicrosftConfig(config *DexConfig) ([]byte, error) {
	return nil, errors.New("Not configured")
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
