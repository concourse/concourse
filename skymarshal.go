package skymarshal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/flag"
	"github.com/concourse/skymarshal/dexserver"
	"github.com/concourse/skymarshal/legacyserver"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/concourse/skymarshal/skyserver"
	"github.com/concourse/skymarshal/token"
)

type Config struct {
	Logger       lager.Logger
	TeamFactory  db.TeamFactory
	Flags        skycmd.AuthFlags
	ExternalURL  string
	InternalHost string
	HttpClient   *http.Client
	Postgres     flag.PostgresConfig
}

type Server struct {
	http.Handler
	*rsa.PrivateKey
}

func (self *Server) PublicKey() *rsa.PublicKey {
	return &self.PrivateKey.PublicKey
}

func NewServer(config *Config) (*Server, error) {
	signingKey, err := loadOrGenerateSigningKey(config.Flags.SigningKey)
	if err != nil {
		return nil, err
	}

	externalURL, err := url.Parse(config.ExternalURL)
	if err != nil {
		return nil, err
	}

	clientId := "skymarshal"
	clientSecretBytes := sha256.Sum256(signingKey.D.Bytes())
	clientSecret := fmt.Sprintf("%x", clientSecretBytes[:])

	issuerPath := "/sky/issuer"
	issuerURL := externalURL.String() + issuerPath
	redirectURL := externalURL.String() + "/sky/callback"

	tokenVerifier := token.NewVerifier(clientId, issuerURL)
	tokenIssuer := token.NewIssuer(config.TeamFactory, token.NewGenerator(signingKey), config.Flags.Expiration)

	internalURL, err := url.Parse(issuerURL)
	if err != nil {
		return nil, err
	}

	internalURL.Host = config.InternalHost

	skyServer, err := skyserver.NewSkyServer(&skyserver.SkyConfig{
		Logger:               config.Logger.Session("sky"),
		TokenVerifier:        tokenVerifier,
		TokenIssuer:          tokenIssuer,
		SigningKey:           signingKey,
		DexExternalIssuerURL: issuerURL,
		DexInternalIssuerURL: internalURL.String(),
		DexClientID:          clientId,
		DexClientSecret:      clientSecret,
		DexRedirectURL:       redirectURL,
		DexHttpClient:        config.HttpClient,
		SecureCookies:        config.Flags.SecureCookies,
	})
	if err != nil {
		return nil, err
	}

	dexServer, err := dexserver.NewDexServer(&dexserver.DexConfig{
		Logger:       config.Logger.Session("dex"),
		Flags:        config.Flags,
		IssuerURL:    issuerURL,
		WebHostURL:   issuerPath,
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Postgres:     config.Postgres,
	})
	if err != nil {
		return nil, err
	}

	legacyServer, err := legacyserver.NewLegacyServer(&legacyserver.LegacyConfig{
		Logger: config.Logger.Session("legacy"),
	})
	if err != nil {
		return nil, err
	}

	handler := http.NewServeMux()
	handler.Handle("/sky/issuer/", dexServer)
	handler.Handle("/sky/", skyserver.NewSkyHandler(skyServer))
	handler.Handle("/auth/", legacyServer)
	handler.Handle("/login", legacyServer)
	handler.Handle("/logout", legacyServer)

	return &Server{handler, signingKey}, nil
}

func loadOrGenerateSigningKey(keyFlag *flag.PrivateKey) (*rsa.PrivateKey, error) {
	if keyFlag != nil && keyFlag.PrivateKey != nil {
		return keyFlag.PrivateKey, nil
	}

	return rsa.GenerateKey(rand.Reader, 2048)
}
