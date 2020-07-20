package skyserver

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/skymarshal/token"
	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2/jwt"
)

type SkyConfig struct {
	Logger          lager.Logger
	TokenMiddleware token.Middleware
	OAuthConfig     *oauth2.Config
	HTTPClient      *http.Client
}

func NewSkyHandler(server *SkyServer) http.Handler {
	handler := http.NewServeMux()
	handler.HandleFunc("/sky/login", server.Login)
	handler.HandleFunc("/sky/logout", server.Logout)
	handler.HandleFunc("/sky/callback", server.Callback)
	return handler
}

func NewSkyServer(config *SkyConfig) (*SkyServer, error) {
	return &SkyServer{config}, nil
}

type SkyServer struct {
	config *SkyConfig
}

func (s *SkyServer) Login(w http.ResponseWriter, r *http.Request) {

	logger := s.config.Logger.Session("login")

	tokenString := s.config.TokenMiddleware.GetAuthToken(r)
	if tokenString == "" {
		s.NewLogin(w, r)
		return
	}

	redirectURI := r.FormValue("redirect_uri")
	if redirectURI == "" {
		redirectURI = "/"
	}

	parts := strings.Split(tokenString, " ")

	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		logger.Info("failed-to-parse-cookie")
		s.NewLogin(w, r)
		return
	}

	parsed, err := jwt.ParseSigned(parts[1])
	if err != nil {
		logger.Error("failed-to-parse-cookie-token", err)
		s.NewLogin(w, r)
		return
	}

	var claims jwt.Claims

	if err = parsed.UnsafeClaimsWithoutVerification(&claims); err != nil {
		logger.Error("failed-to-parse-claims", err)
		s.NewLogin(w, r)
		return
	}

	if err = claims.Validate(jwt.Expected{Time: time.Now()}); err != nil {
		logger.Error("failed-to-validate-claims", err)
		s.NewLogin(w, r)
		return
	}

	oauth2Token := &oauth2.Token{
		TokenType:   parts[0],
		AccessToken: parts[1],
		Expiry:      claims.Expiry.Time(),
	}

	s.Redirect(w, r, oauth2Token, redirectURI)
}

func (s *SkyServer) NewLogin(w http.ResponseWriter, r *http.Request) {

	logger := s.config.Logger.Session("new-login")

	redirectURI := r.FormValue("redirect_uri")
	if redirectURI == "" {
		redirectURI = "/"
	}

	stateToken := encode(stateToken{
		RedirectURI: redirectURI,
		Entropy:     randomString(),
	})

	err := s.config.TokenMiddleware.SetStateToken(w, stateToken, time.Now().Add(time.Hour))
	if err != nil {
		logger.Error("invalid-state-token", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	authCodeURL := s.config.OAuthConfig.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)

	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}

func (s *SkyServer) Callback(w http.ResponseWriter, r *http.Request) {

	logger := s.config.Logger.Session("callback")

	if errMsg, errDesc := r.FormValue("error"), r.FormValue("error_description"); errMsg != "" {
		logger.Error("failed-with-callback-error", errors.New(errMsg+" : "+errDesc))
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	stateToken := s.config.TokenMiddleware.GetStateToken(r)
	if stateToken == "" {
		logger.Error("failed-with-invalid-state-token", errors.New("state token is empty"))
		http.Error(w, "invalid state token", http.StatusBadRequest)
		return
	}

	if stateToken != r.FormValue("state") {
		logger.Error("failed-with-unexpected-state-token", errors.New("state token does not match"))
		http.Error(w, "unexpected state token", http.StatusBadRequest)
		return
	}

	s.config.TokenMiddleware.UnsetStateToken(w)

	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, s.config.HTTPClient)

	dexToken, err := s.config.OAuthConfig.Exchange(ctx, r.FormValue("code"))
	if err != nil {
		logger.Error("failed-to-fetch-dex-token", err)
		switch e := err.(type) {
		case *oauth2.RetrieveError:
			http.Error(w, string(e.Body), e.Response.StatusCode)
			return
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	s.Redirect(w, r, dexToken, decode(stateToken).RedirectURI)
}

func (s *SkyServer) Redirect(w http.ResponseWriter, r *http.Request, oauth2Token *oauth2.Token, redirectURI string) {
	logger := s.config.Logger.Session("redirect")

	redirectURL, err := url.ParseRequestURI(redirectURI)
	if err != nil {
		logger.Error("failed-to-parse-redirect-url", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if redirectURL.Host != "" {
		logger.Error("invalid-redirect", fmt.Errorf("Unsupported redirect uri: %s", redirectURI))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = s.config.TokenMiddleware.SetAuthToken(w, oauth2Token.TokenType+" "+oauth2Token.AccessToken, oauth2Token.Expiry)
	if err != nil {
		logger.Error("failed-to-set-auth-token", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	csrfToken := randomString()

	err = s.config.TokenMiddleware.SetCSRFToken(w, csrfToken, oauth2Token.Expiry)
	if err != nil {
		logger.Error("failed-to-set-state-token", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	params := redirectURL.Query()
	params.Set("csrf_token", csrfToken)

	http.Redirect(w, r, redirectURL.EscapedPath()+"?"+params.Encode(), http.StatusTemporaryRedirect)
}

func (s *SkyServer) Logout(w http.ResponseWriter, r *http.Request) {
	s.config.TokenMiddleware.UnsetAuthToken(w)
	s.config.TokenMiddleware.UnsetCSRFToken(w)
}

type stateToken struct {
	RedirectURI string `json:"redirect_uri"`
	Entropy     string `json:"entropy"`
}

func encode(token stateToken) string {
	json, _ := json.Marshal(token)

	return base64.StdEncoding.EncodeToString(json)
}

func decode(raw string) stateToken {
	data, _ := base64.StdEncoding.DecodeString(raw)

	var token stateToken
	json.Unmarshal(data, &token)
	return token
}

func randomString() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
