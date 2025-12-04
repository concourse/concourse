package skyserver

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/go-jose/go-jose/v4/jwt"
	"golang.org/x/oauth2"
)

type SkyConfig struct {
	Logger             lager.Logger
	TokenMiddleware    token.Middleware
	TokenParser        token.Parser
	OAuthConfig        *oauth2.Config
	HTTPClient         *http.Client
	AccessTokenFactory db.AccessTokenFactory
	ClaimsCacher       accessor.AccessTokenFetcher
	StateSigningKey    []byte
}

func NewSkyHandler(server *SkyServer) http.Handler {
	handler := http.NewServeMux()
	handler.HandleFunc("/sky/login", server.Login)
	handler.HandleFunc("/sky/logout", server.Logout)
	handler.HandleFunc("/sky/callback", server.Callback)
	return handler
}

func NewSkyServer(config *SkyConfig) (*SkyServer, error) {
	if len(config.StateSigningKey) < 32 {
		return nil, errors.New("StateSigningKey must be at least 32 bytes")
	}
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

	expiry, err := s.config.TokenParser.ParseExpiry(parts[1])
	if err != nil {
		logger.Error("failed-to-parse-expiration", err)
		s.NewLogin(w, r)
		return
	}
	nowWithLeeway := time.Now().Add(-jwt.DefaultLeeway)
	if expiry.Before(nowWithLeeway) {
		logger.Info("token-is-expired")
		s.NewLogin(w, r)
		return
	}

	oauth2Token := &oauth2.Token{
		TokenType:   parts[0],
		AccessToken: parts[1],
		Expiry:      expiry,
	}

	s.Redirect(w, r, oauth2Token, redirectURI)
}

func (s *SkyServer) NewLogin(w http.ResponseWriter, r *http.Request) {

	logger := s.config.Logger.Session("new-login")

	redirectURI := r.FormValue("redirect_uri")
	if redirectURI == "" {
		redirectURI = "/"
	}

	stateToken, err := s.signState(stateToken{
		RedirectURI: redirectURI,
		Entropy:     randomString(),
		Timestamp:   time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-sign-state", err)
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

	urlState := r.FormValue("state")
	state, err := s.verifyState(urlState)
	if err != nil {
		logger.Error("failed-to-verify-state", err)
		http.Error(w, "invalid state token", http.StatusBadRequest)
		return
	}

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

	s.Redirect(w, r, dexToken, state.RedirectURI)
}

func (s *SkyServer) Redirect(w http.ResponseWriter, r *http.Request, oauth2Token *oauth2.Token, redirectURI string) {
	logger := s.config.Logger.Session("redirect")

	redirectURL, err := url.ParseRequestURI("/" + strings.TrimLeft(redirectURI, "/"))
	if err != nil {
		logger.Error("failed-to-parse-redirect-url", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if redirectURL.Host != "" || redirectURL.Scheme != "" {
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
	logger := s.config.Logger.Session("logout")

	tokenString := s.config.TokenMiddleware.GetAuthToken(r)
	if tokenString == "" {
		logger.Debug("no-auth-token")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	parts := strings.Split(tokenString, " ")

	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		logger.Info("failed-to-parse-auth-token")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err := s.config.ClaimsCacher.DeleteAccessToken(parts[1])
	if err != nil {
		logger.Error("delete-auth-token-from-cache", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.config.AccessTokenFactory.DeleteAccessToken(parts[1])
	if err != nil {
		logger.Error("delete-auth-token-from-db", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.config.TokenMiddleware.UnsetAuthToken(w)
	s.config.TokenMiddleware.UnsetCSRFToken(w)
}

type stateToken struct {
	RedirectURI string `json:"redirect_uri"`
	Entropy     string `json:"entropy"`
	Timestamp   int64  `json:"ts"`
	Signature   string `json:"sig,omitempty"`
}

const stateTokenMaxAge = 3600 // 1 hour

func (s *SkyServer) signState(st stateToken) (string, error) {
	st.Signature = ""
	payload, err := json.Marshal(st)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, s.config.StateSigningKey)
	mac.Write(payload)
	st.Signature = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	signed, err := json.Marshal(st)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(signed), nil
}

func (s *SkyServer) verifyState(raw string) (stateToken, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return stateToken{}, errors.New("failed to decode state")
	}

	var st stateToken
	if err := json.Unmarshal(data, &st); err != nil {
		return stateToken{}, errors.New("failed to unmarshal state")
	}

	sig := st.Signature
	if sig == "" {
		return stateToken{}, errors.New("missing signature")
	}

	st.Signature = ""
	payload, err := json.Marshal(st)
	if err != nil {
		return stateToken{}, errors.New("failed to marshal state for verification")
	}

	mac := hmac.New(sha256.New, s.config.StateSigningKey)
	mac.Write(payload)
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return stateToken{}, errors.New("signature mismatch")
	}

	if time.Now().Unix()-st.Timestamp > stateTokenMaxAge {
		return stateToken{}, errors.New("state expired")
	}

	return st, nil
}

func randomString() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
