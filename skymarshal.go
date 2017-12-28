package skymarshal

import (
	"crypto/rsa"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/authserver"
)

type Config struct {
	BaseUrl      string
	BaseAuthUrl  string
	SigningKey   *rsa.PrivateKey
	Expiration   time.Duration
	IsTLSEnabled bool
	TeamFactory  db.TeamFactory
	Logger       lager.Logger
}

type DetailedConfig struct {
	BaseUrl            string
	BaseAuthUrl        string
	Expiration         time.Duration
	IsTLSEnabled       bool
	TeamFactory        db.TeamFactory
	Logger             lager.Logger
	OAuthFactory       auth.ProviderFactory
	OAuthFactoryV1     auth.ProviderFactory
	TokenReader        auth.TokenReader
	TokenValidator     auth.TokenValidator
	BasicAuthValidator auth.TokenValidator
	CSRFTokenGenerator auth.CSRFTokenGenerator
	AuthTokenGenerator auth.AuthTokenGenerator
}

func NewHandler(config *Config) (http.Handler, error) {

	config.Logger = config.Logger.Session("skymarshal")

	oauthFactory := auth.NewOAuthFactory(
		config.Logger.Session("oauth-provider-factory"),
		config.BaseAuthUrl,
		auth.Routes,
		auth.OAuthCallback,
	)

	oauthFactoryV1 := auth.NewOAuthFactory(
		config.Logger.Session("oauth-v1-provider-factory"),
		config.BaseAuthUrl,
		auth.V1Routes,
		auth.OAuthV1Callback,
	)

	return NewHandlerWithOptions(&DetailedConfig{
		config.BaseUrl,
		config.BaseAuthUrl,
		config.Expiration,
		config.IsTLSEnabled,
		config.TeamFactory,
		config.Logger,
		oauthFactory,
		oauthFactoryV1,
		auth.JWTReader{&config.SigningKey.PublicKey},
		auth.JWTValidator{&config.SigningKey.PublicKey},
		auth.NewBasicAuthValidator(config.Logger, config.TeamFactory),
		auth.NewCSRFTokenGenerator(),
		auth.NewAuthTokenGenerator(config.SigningKey),
	})
}

func NewHandlerWithOptions(config *DetailedConfig) (http.Handler, error) {

	oauthHandler, err := auth.NewOAuthHandler(
		config.Logger,
		config.OAuthFactory,
		config.TeamFactory,
		config.CSRFTokenGenerator,
		config.AuthTokenGenerator,
		config.Expiration,
		config.IsTLSEnabled,
	)
	if err != nil {
		return nil, err
	}

	oauthV1Handler, err := auth.NewOAuthV1Handler(
		config.Logger,
		config.OAuthFactoryV1,
		config.TeamFactory,
		config.CSRFTokenGenerator,
		config.AuthTokenGenerator,
		config.Expiration,
		config.IsTLSEnabled,
	)
	if err != nil {
		return nil, err
	}

	authServer := authserver.NewServer(
		config.Logger,
		config.BaseUrl,
		config.BaseAuthUrl,
		config.Expiration,
		config.IsTLSEnabled,
		config.TeamFactory,
		config.OAuthFactory,
		config.CSRFTokenGenerator,
		config.AuthTokenGenerator,
		config.TokenReader,
		config.TokenValidator,
		config.BasicAuthValidator,
	)

	webMux := http.NewServeMux()
	webMux.Handle("/auth/list_methods", http.HandlerFunc(authServer.ListAuthMethods))
	webMux.Handle("/auth/basic/token", http.HandlerFunc(authServer.GetAuthToken))
	webMux.Handle("/auth/userinfo", http.HandlerFunc(authServer.GetUser))
	webMux.Handle("/auth/", oauthHandler)
	webMux.Handle("/oauth/v1/", oauthV1Handler)
	return webMux, nil
}
