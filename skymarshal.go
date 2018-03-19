package skymarshal

import (
	"crypto/rsa"
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/dexserver"
	"github.com/concourse/skymarshal/skyserver"
	"github.com/concourse/skymarshal/token"
)

type Config struct {
	Logger             lager.Logger
	TeamFactory        db.TeamFactory
	TLSConfig          *tls.Config
	SigningKey         *rsa.PrivateKey
	Expiration         time.Duration
	DexServerURL       string
	SkyServerURL       string
	GithubClientID     string
	GithubClientSecret string
	LocalUsers         map[string]string
}

type Server struct {
	http.Handler
}

func NewServer(config *Config) (*Server, error) {

	clientId := "skymarshal"
	clientSecret := token.RandomString()

	issuerUrl := strings.TrimRight(config.DexServerURL, "/") + "/sky/dex"
	redirectUrl := strings.TrimRight(config.SkyServerURL, "/") + "/sky/callback"

	tokenVerifier := token.NewVerifier(clientId, issuerUrl)
	tokenGenerator := token.NewGenerator(config.SigningKey)
	tokenIssuer := token.NewIssuer(config.TeamFactory, tokenGenerator, config.Expiration)

	skyServer, err := skyserver.NewSkyServer(&skyserver.SkyConfig{
		Logger:          config.Logger,
		TokenVerifier:   tokenVerifier,
		TokenIssuer:     tokenIssuer,
		SigningKey:      config.SigningKey,
		TLSConfig:       config.TLSConfig,
		DexIssuerURL:    issuerUrl,
		DexClientID:     clientId,
		DexClientSecret: clientSecret,
		DexRedirectURL:  redirectUrl,
	})
	if err != nil {
		return nil, err
	}

	dexServer, err := dexserver.NewDexServer(&dexserver.DexConfig{
		GithubClientID:     config.GithubClientID,
		GithubClientSecret: config.GithubClientSecret,
		LocalUsers:         config.LocalUsers,
		IssuerURL:          issuerUrl,
		ClientID:           clientId,
		ClientSecret:       clientSecret,
		RedirectURL:        redirectUrl,
	})
	if err != nil {
		return nil, err
	}

	handler := http.NewServeMux()
	handler.Handle("/sky/dex/", dexServer)
	handler.Handle("/sky/", skyserver.NewSkyHandler(skyServer))
	return &Server{handler}, nil
}
