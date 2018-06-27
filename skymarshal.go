package skymarshal

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/flag"
	"github.com/concourse/skymarshal/dexserver"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/concourse/skymarshal/skyserver"
	"github.com/concourse/skymarshal/token"
)

type Config struct {
	Logger      lager.Logger
	TeamFactory db.TeamFactory
	Flags       skycmd.AuthFlags
	ServerURL   string
	HttpClient  *http.Client
}

type Server struct {
	http.Handler
	*rsa.PrivateKey
}

func (self *Server) PublicKey() *rsa.PublicKey {
	return &self.PrivateKey.PublicKey
}

func NewServer(config *Config) (*Server, error) {

	clientId := "skymarshal"
	clientSecret := token.RandomString()

	signingKey, err := loadOrGenerateSigningKey(config.Flags.SigningKey)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(config.ServerURL)
	if err != nil {
		return nil, err
	}

	issuerUrl := serverURL.String() + "/sky/issuer"
	redirectUrl := serverURL.String() + "/sky/callback"

	tokenVerifier := token.NewVerifier(clientId, issuerUrl)
	tokenIssuer := token.NewIssuer(config.TeamFactory, token.NewGenerator(signingKey), config.Flags.Expiration)

	skyServer, err := skyserver.NewSkyServer(&skyserver.SkyConfig{
		Logger:          config.Logger.Session("sky"),
		TokenVerifier:   tokenVerifier,
		TokenIssuer:     tokenIssuer,
		SigningKey:      signingKey,
		DexIssuerURL:    issuerUrl,
		DexClientID:     clientId,
		DexClientSecret: clientSecret,
		DexRedirectURL:  redirectUrl,
		DexHttpClient:   config.HttpClient,
		SecureCookies:   config.Flags.SecureCookies,
	})
	if err != nil {
		return nil, err
	}

	dexServer, err := dexserver.NewDexServer(&dexserver.DexConfig{
		Logger:       config.Logger.Session("dex"),
		Flags:        config.Flags,
		IssuerURL:    issuerUrl,
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUrl,
	})
	if err != nil {
		return nil, err
	}

	handler := http.NewServeMux()
	handler.Handle("/sky/issuer/", dexServer)
	handler.Handle("/sky/", skyserver.NewSkyHandler(skyServer))

	return &Server{handler, signingKey}, nil
}

func loadOrGenerateSigningKey(keyFlag *flag.PrivateKey) (*rsa.PrivateKey, error) {
	if keyFlag != nil && keyFlag.PrivateKey != nil {
		return keyFlag.PrivateKey, nil
	}

	return rsa.GenerateKey(rand.Reader, 2048)
}
