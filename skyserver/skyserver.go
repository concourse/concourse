package skyserver

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
		http.Error(w, fmt.Sprintf("Fetching cookie state failed: %v", err), http.StatusBadRequest)
		return
	}

	if errMsg := r.FormValue("error"); errMsg != "" {
		http.Error(w, errMsg+": "+r.FormValue("error_description"), http.StatusBadRequest)
		return
	}

	if stateToken = cookieState.Value; stateToken != r.FormValue("state") {
		http.Error(w, fmt.Sprintf("Unexpected state token"), http.StatusBadRequest)
		return
	}

	if authCode = r.FormValue("code"); authCode == "" {
		http.Error(w, fmt.Sprintf("No auth code in request"), http.StatusBadRequest)
		return
	}

	ctx := oidc.ClientContext(r.Context(), self.config.DexHttpClient)

	if dexToken, err = oauth2Config.Exchange(ctx, authCode); err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch dex token: %v", err), http.StatusInternalServerError)
		return
	}

	if verifiedClaims, err = self.config.TokenVerifier.Verify(ctx, dexToken); err != nil {
		http.Error(w, fmt.Sprintf("Failed to verify dex token: %v", err), http.StatusBadRequest)
		return
	}

	if skyToken, err = self.config.TokenIssuer.Issue(verifiedClaims); err != nil {
		http.Error(w, fmt.Sprintf("Failed to issue concourse token: %v", err), http.StatusBadRequest)
		return
	}

	tokenStr := skyToken.TokenType + " " + skyToken.AccessToken

	csrfToken, ok := skyToken.Extra("csrf").(string)
	if !ok {
		http.Error(w, "missing csrf", http.StatusBadRequest)
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
		http.Error(w, "invalid redirect", http.StatusBadRequest)
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

	var (
		err                error
		grantType, scopes  string
		username, password string
		dexToken, skyToken *oauth2.Token
		verifiedClaims     *token.VerifiedClaims
	)

	clientId, clientSecret, ok := r.BasicAuth()
	if !ok {
		http.Error(w, "invalid basic auth", http.StatusUnauthorized)
		return
	}

	if clientId != "fly" || clientSecret != "Zmx5" {
		http.Error(w, "invalid client", http.StatusBadRequest)
		return
	}

	if grantType = r.FormValue("grant_type"); grantType != "password" {
		http.Error(w, "invalid grant type", http.StatusBadRequest)
		return
	}

	if username = r.FormValue("username"); username == "" {
		http.Error(w, "missing username", http.StatusBadRequest)
		return
	}

	if password = r.FormValue("password"); password == "" {
		http.Error(w, "missing password", http.StatusBadRequest)
		return
	}

	if scopes = r.FormValue("scope"); scopes == "" {
		http.Error(w, "missing scopes", http.StatusBadRequest)
		return
	}

	oauth2Config := &oauth2.Config{
		ClientID:     self.config.DexClientID,
		ClientSecret: self.config.DexClientSecret,
		Endpoint:     self.endpoint(),
		Scopes:       strings.Split(scopes, "+"),
	}

	ctx := oidc.ClientContext(r.Context(), self.config.DexHttpClient)

	if dexToken, err = oauth2Config.PasswordCredentialsToken(ctx, username, password); err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch dex token: %v", err), http.StatusInternalServerError)
		return
	}

	if verifiedClaims, err = self.config.TokenVerifier.Verify(ctx, dexToken); err != nil {
		http.Error(w, fmt.Sprintf("Failed to verify dex token: %v", err), http.StatusBadRequest)
		return
	}

	if skyToken, err = self.config.TokenIssuer.Issue(verifiedClaims); err != nil {
		http.Error(w, fmt.Sprintf("Failed to issue concourse token: %v", err), http.StatusBadRequest)
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

	parts := strings.Split(r.Header.Get("Authorization"), " ")

	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	parsed, err := jwt.ParseSigned(parts[1])
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var result map[string]interface{}
	if err = parsed.Claims(&self.config.SigningKey.PublicKey, &result); err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
