package skymarshal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/skymarshal/dexserver"
	"github.com/concourse/concourse/skymarshal/legacyserver"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/skymarshal/skyserver"
	"github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/flag"
)

type Config struct {
	Logger      lager.Logger
	TeamFactory db.TeamFactory
	Flags       skycmd.AuthFlags
	ExternalURL string
	HTTPClient  *http.Client
	Storage     storage.Storage
}

type Server struct {
	http.Handler
	*rsa.PrivateKey
}

func (s *Server) PublicKey() *rsa.PublicKey {
	return &s.PrivateKey.PublicKey
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

	clientID := "skymarshal"
	clientSecretBytes := sha256.Sum256(signingKey.D.Bytes())
	clientSecret := fmt.Sprintf("%x", clientSecretBytes[:])

	issuerPath := "/sky/issuer"
	issuerURL := externalURL.String() + issuerPath
	redirectURL := externalURL.String() + "/sky/callback"

	tokenVerifier := token.NewVerifier(clientID, issuerURL)
	tokenIssuer := token.NewIssuer(config.TeamFactory, token.newGenerator(signingKey), config.Flags.Expiration)

	skyServer, err := skyserver.NewSkyServer(&skyserver.SkyConfig{
		Logger:          config.Logger.Session("sky"),
		TokenVerifier:   tokenVerifier,
		TokenIssuer:     tokenIssuer,
		SigningKey:      signingKey,
		DexIssuerURL:    issuerURL,
		DexClientID:     clientID,
		DexClientSecret: clientSecret,
		DexRedirectURL:  redirectURL,
		DexHTTPClient:   config.HTTPClient,
		SecureCookies:   config.Flags.SecureCookies,
	})
	if err != nil {
		return nil, err
	}

	dexServer, err := dexserver.NewDexServer(&dexserver.DexConfig{
		Logger:       config.Logger.Session("dex"),
		Flags:        config.Flags,
		IssuerURL:    issuerURL,
		WebHostURL:   issuerPath,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Storage:      config.Storage,
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
