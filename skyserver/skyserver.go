package skyserver

import (
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/token"
	"github.com/coreos/go-oidc"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/oauth2"
)

type SkyConfig struct {
	Logger          lager.Logger
	TokenVerifier   token.Verifier
	TokenIssuer     token.Issuer
	SigningKey      *rsa.PrivateKey
	TLSConfig       *tls.Config
	DexClientID     string
	DexClientSecret string
	DexRedirectURL  string
	DexIssuerURL    string
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

	httpClient := http.DefaultClient

	if config.TLSConfig != nil {
		httpClient.Transport = &http.Transport{TLSClientConfig: config.TLSConfig}
	}

	return &skyServer{
		client: httpClient,
		config: config,
	}, nil
}

type skyServer struct {
	client *http.Client
	config *SkyConfig
}

func (self *skyServer) UserInfo(w http.ResponseWriter, r *http.Request) {

	token, err := getJWT(r, &self.config.SigningKey.PublicKey)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(token.Claims)
}

func (self *skyServer) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   authCookieName,
		Path:   "/",
		MaxAge: -1,
	})
}

func (self *skyServer) Login(w http.ResponseWriter, r *http.Request) {

	oauth2Config := &oauth2.Config{
		ClientID:     self.config.DexClientID,
		ClientSecret: self.config.DexClientSecret,
		RedirectURL:  self.config.DexRedirectURL,
		Endpoint:     self.endpoint(),
		Scopes:       []string{"openid", "profile", "email", "federated:id", "groups"},
	}

	stateToken := encode(&token.StateToken{
		RedirectUri: r.FormValue("redirect_uri"),
		Entropy:     token.RandomString(),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    stateToken,
		Path:     "/",
		Expires:  time.Now().Add(time.Minute),
		Secure:   self.secure(),
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

	ctx := oidc.ClientContext(r.Context(), self.client)

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

	tokenStr := string(skyToken.TokenType) + " " + skyToken.AccessToken

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
		Secure:   self.secure(),
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
		ClientID:     clientId,
		ClientSecret: clientSecret,
		Endpoint:     self.endpoint(),
		Scopes:       strings.Split(scopes, "+"),
	}

	ctx := oidc.ClientContext(r.Context(), self.client)

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

	data, _ := json.Marshal(skyToken)

	w.Write(data)
}

func (self *skyServer) endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:  self.config.DexIssuerURL + "/auth",
		TokenURL: self.config.DexIssuerURL + "/token",
	}
}

func (self *skyServer) secure() bool {
	return self.config.TLSConfig != nil
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

func getJWT(r *http.Request, publicKey *rsa.PublicKey) (token *jwt.Token, err error) {
	fun := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return publicKey, nil
	}

	if ah := r.Header.Get("Authorization"); ah != "" {
		if len(ah) > 6 && strings.ToUpper(ah[0:6]) == "BEARER" {
			return jwt.Parse(ah[7:], fun)
		}
	}

	return nil, errors.New("unable to parse authorization header")
}
