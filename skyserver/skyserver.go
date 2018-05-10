package skyserver

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/token"
	"github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2/jwt"
)

type SkyConfig struct {
	Logger          lager.Logger
	TokenVerifier   token.Verifier
	TokenIssuer     token.Issuer
	SigningKey      *rsa.PrivateKey
	SecureCookies   bool
	DexClientID     string
	DexClientSecret string
	DexRedirectURL  string
	DexIssuerURL    string
	DexHttpClient   *http.Client
}

const stateCookieName = "skymarshal_state"
const authCookieName = "skymarshal_auth"

func NewSkyHandler(server *skyServer) http.Handler {
	handler := http.NewServeMux()
	handler.HandleFunc("/sky/login", server.Login)
	handler.HandleFunc("/sky/logout", server.Logout)
	handler.HandleFunc("/sky/callback", server.Callback)
	handler.HandleFunc("/sky/userinfo", server.UserInfo)
	handler.HandleFunc("/sky/token", server.Token)
	return handler
}

func NewSkyServer(config *SkyConfig) (*skyServer, error) {
	return &skyServer{config}, nil
}

type skyServer struct {
	config *SkyConfig
}

func (self *skyServer) Login(w http.ResponseWriter, r *http.Request) {

	oauth2Config := &oauth2.Config{
		ClientID:     self.config.DexClientID,
		ClientSecret: self.config.DexClientSecret,
		RedirectURL:  self.config.DexRedirectURL,
		Endpoint:     self.endpoint(),
		Scopes:       []string{"openid", "profile", "email", "federated:id", "groups"},
	}

	redirectUri := r.FormValue("redirect_uri")
	if redirectUri == "" {
		redirectUri = "/"
	}

	stateToken := encode(&token.StateToken{
		RedirectUri: redirectUri,
		Entropy:     token.RandomString(),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    stateToken,
		Path:     "/",
		Expires:  time.Now().Add(time.Minute),
		Secure:   self.config.SecureCookies,
		HttpOnly: true,
	})

	authCodeURL := oauth2Config.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)

	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}

func (self *skyServer) Callback(w http.ResponseWriter, r *http.Request) {

	logger := self.config.Logger.Session("callback")

	var (
		err                  error
		stateToken, authCode string
		dexToken, skyToken   *oauth2.Token
		verifiedClaims       *token.VerifiedClaims
	)

	oauth2Config := &oauth2.Config{
		ClientID:     self.config.DexClientID,
		ClientSecret: self.config.DexClientSecret,
		RedirectURL:  self.config.DexRedirectURL,
		Endpoint:     self.endpoint(),
	}

	cookieState, err := r.Cookie(stateCookieName)
	if err != nil {
		logger.Error("failed-to-fetch-cookie-state", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if errMsg, errDesc := r.FormValue("error"), r.FormValue("error_description"); errMsg != "" {
		logger.Error("failed-with-callback-error", errors.New(errMsg+" : "+errDesc))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if stateToken = cookieState.Value; stateToken != r.FormValue("state") {
		logger.Error("failed-with-unexpected-state-token", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if authCode = r.FormValue("code"); authCode == "" {
		logger.Error("failed-to-get-auth-code", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := oidc.ClientContext(r.Context(), self.config.DexHttpClient)

	if dexToken, err = oauth2Config.Exchange(ctx, authCode); err != nil {
		logger.Error("failed-to-fetch-dex-token", err)
		switch e := err.(type) {
		case *oauth2.RetrieveError:
			http.Error(w, string(e.Body), e.Response.StatusCode)
			return
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if verifiedClaims, err = self.config.TokenVerifier.Verify(ctx, dexToken); err != nil {
		logger.Error("failed-to-verify-dex-token", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if skyToken, err = self.config.TokenIssuer.Issue(verifiedClaims); err != nil {
		logger.Error("failed-to-issue-concourse-token", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tokenStr := skyToken.TokenType + " " + skyToken.AccessToken

	csrfToken, ok := skyToken.Extra("csrf").(string)
	if !ok {
		logger.Error("failed-to-include-csrf-token", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    tokenStr,
		Path:     "/",
		Expires:  skyToken.Expiry,
		HttpOnly: true,
		Secure:   self.config.SecureCookies,
	})

	http.SetCookie(w, &http.Cookie{
		Name:   stateCookieName,
		Path:   "/",
		MaxAge: -1,
	})

	redirectUrl, err := url.Parse(decode(stateToken).RedirectUri)
	if err != nil {
		logger.Error("failed-to-parse-redirect-url", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	params := redirectUrl.Query()
	params.Set("token", tokenStr)
	params.Set("csrf_token", csrfToken)

	redirectUrl.RawQuery = params.Encode()

	w.Header().Set("X-Csrf-Token", csrfToken)

	http.Redirect(w, r, redirectUrl.String(), http.StatusTemporaryRedirect)
}

func (self *skyServer) Token(w http.ResponseWriter, r *http.Request) {

	logger := self.config.Logger.Session("token")

	var (
		err                error
		grantType          string
		dexToken, skyToken *oauth2.Token
		verifiedClaims     *token.VerifiedClaims
	)

	clientId, clientSecret, ok := r.BasicAuth()
	if !ok {
		logger.Error("invalid-basic-auth", nil)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if clientId != "fly" || clientSecret != "Zmx5" {
		logger.Error("invalid-client", nil)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if grantType = r.FormValue("grant_type"); grantType != "password" {
		logger.Error("invalid-grant-type", nil, lager.Data{"grant_type": grantType})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	scope := r.FormValue("scope")

	oauth2Config := &oauth2.Config{
		ClientID:     self.config.DexClientID,
		ClientSecret: self.config.DexClientSecret,
		Endpoint:     self.endpoint(),
		Scopes:       strings.Split(scope, "+"),
	}

	ctx := oidc.ClientContext(r.Context(), self.config.DexHttpClient)

	if dexToken, err = oauth2Config.PasswordCredentialsToken(ctx, username, password); err != nil {
		logger.Error("failed-to-fetch-dex-token", err)
		switch e := err.(type) {
		case *oauth2.RetrieveError:
			http.Error(w, string(e.Body), e.Response.StatusCode)
			return
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if verifiedClaims, err = self.config.TokenVerifier.Verify(ctx, dexToken); err != nil {
		logger.Error("failed-to-verify-dex-token", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if skyToken, err = self.config.TokenIssuer.Issue(verifiedClaims); err != nil {
		logger.Error("failed-to-issue-concourse-token", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(skyToken)
}

func (self *skyServer) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   authCookieName,
		Path:   "/",
		MaxAge: -1,
	})
}

func (self *skyServer) UserInfo(w http.ResponseWriter, r *http.Request) {

	logger := self.config.Logger.Session("userinfo")

	parts := strings.Split(r.Header.Get("Authorization"), " ")

	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		logger.Info("faild-to-parse-authorization-header")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	parsed, err := jwt.ParseSigned(parts[1])
	if err != nil {
		logger.Error("failed-to-parse-authorization-token", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var claims jwt.Claims
	var result map[string]interface{}

	if err = parsed.Claims(&self.config.SigningKey.PublicKey, &claims, &result); err != nil {
		logger.Error("failed-to-parse-claims", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err = claims.Validate(jwt.Expected{Time: time.Now()}); err != nil {
		logger.Error("failed-to-validate-claims", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(result)
}

func (self *skyServer) endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:  strings.TrimRight(self.config.DexIssuerURL, "/") + "/auth",
		TokenURL: strings.TrimRight(self.config.DexIssuerURL, "/") + "/token",
	}
}

func encode(token *token.StateToken) string {
	json, _ := json.Marshal(token)

	return base64.StdEncoding.EncodeToString(json)
}

func decode(raw string) *token.StateToken {
	data, _ := base64.StdEncoding.DecodeString(raw)

	var token *token.StateToken
	json.Unmarshal(data, &token)
	return token
}
