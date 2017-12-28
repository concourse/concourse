package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"

	"net/url"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const AuthCookieName = "ATC-Authorization"
const CSRFHeaderName = "X-Csrf-Token"

type OAuthCallbackHandler struct {
	logger             lager.Logger
	providerFactory    ProviderFactory
	teamFactory        db.TeamFactory
	csrfTokenGenerator CSRFTokenGenerator
	authTokenGenerator AuthTokenGenerator
	expire             time.Duration
	isTLSEnabled       bool
	stateValidator     oauthStateValidator
}

func NewOAuthCallbackHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	teamFactory db.TeamFactory,
	csrfTokenGenerator CSRFTokenGenerator,
	authTokenGenerator AuthTokenGenerator,
	expire time.Duration,
	isTLSEnabled bool,
	stateValidator oauthStateValidator,
) http.Handler {
	return &OAuthCallbackHandler{
		logger:             logger,
		providerFactory:    providerFactory,
		teamFactory:        teamFactory,
		csrfTokenGenerator: csrfTokenGenerator,
		authTokenGenerator: authTokenGenerator,
		expire:             expire,
		isTLSEnabled:       isTLSEnabled,
		stateValidator:     stateValidator,
	}
}

func (handler *OAuthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hLog := handler.logger.Session("callback")
	providerName := r.FormValue(":provider")
	paramState := r.FormValue("state")

	cookieState, err := r.Cookie(OAuthStateCookie)
	if err != nil {
		hLog.Info("no-state-cookie", lager.Data{
			"error": err.Error(),
		})
		http.Error(w, "state cookie not set", http.StatusUnauthorized)
		return
	}

	if !handler.stateValidator.Valid(cookieState.Value, paramState) {
		hLog.Info("state-cookie-mismatch", lager.Data{
			"param-state":  paramState,
			"cookie-state": cookieState.Value,
		})

		http.Error(w, "state cookie does not match param", http.StatusUnauthorized)
		return
	}

	// Read the state from the cookie instead of the param, as the param
	// will be empty if this is an OAuth 1 request. For OAuth 2, we already
	// made sure that the cookie and the param contain the same state.
	stateJSON, err := base64.RawURLEncoding.DecodeString(cookieState.Value)
	if err != nil {
		hLog.Info("failed-to-decode-state", lager.Data{
			"error": err.Error(),
		})
		http.Error(w, "state value invalid base64", http.StatusUnauthorized)
		return
	}

	var oauthState OAuthState
	err = json.Unmarshal(stateJSON, &oauthState)
	if err != nil {
		hLog.Info("failed-to-unmarshal-state", lager.Data{
			"error": err.Error(),
		})
		http.Error(w, "state value invalid JSON", http.StatusUnauthorized)
		return
	}

	teamName := oauthState.TeamName
	team, found, err := handler.teamFactory.FindTeam(teamName)

	if err != nil {
		hLog.Error("failed-to-get-team", err)
		http.Error(w, "failed to get team", http.StatusInternalServerError)
		return
	}
	if !found {
		hLog.Info("failed-to-find-team", lager.Data{
			"teamName": teamName,
		})
		http.Error(w, "failed to find team", http.StatusNotFound)
		return
	}

	provider, found, err := handler.providerFactory.GetProvider(team, providerName)
	if err != nil {
		handler.logger.Error("failed-to-get-provider", err, lager.Data{
			"provider": providerName,
			"teamName": teamName,
		})

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		handler.logger.Info("provider-not-found-for-team", lager.Data{
			"provider": providerName,
			"teamName": teamName,
		})

		w.WriteHeader(http.StatusNotFound)
		return
	}

	preTokenClient, err := provider.PreTokenClient()
	if err != nil {
		handler.logger.Error("failed-to-construct-pre-token-client", err, lager.Data{
			"provider": providerName,
			"teamName": teamName,
		})

		http.Error(w, "unable to connect to provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := context.WithValue(oauth2.NoContext, oauth2.HTTPClient, preTokenClient)

	token, err := provider.Exchange(ctx, r)
	if err != nil {
		hLog.Error("failed-to-exchange-token", err)
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	httpClient := provider.Client(ctx, token)

	verified, err := provider.Verify(hLog.Session("verify"), httpClient)
	if err != nil {
		hLog.Error("failed-to-verify-token", err)
		http.Error(w, "failed to verify token", http.StatusInternalServerError)
		return
	}

	if !verified {
		hLog.Info("verification-failed")
		http.Error(w, "verification failed", http.StatusUnauthorized)
		return
	}

	exp := time.Now().Add(handler.expire)

	csrfToken, err := handler.csrfTokenGenerator.GenerateToken()
	if err != nil {
		hLog.Error("generate-csrf-token", err)
		http.Error(w, "failed to generate csrf token", http.StatusInternalServerError)
		return
	}

	tokenType, signedToken, err := handler.authTokenGenerator.GenerateToken(exp, team.Name(), team.Admin(), csrfToken)
	if err != nil {
		hLog.Error("failed-to-sign-token", err)
		http.Error(w, "failed to generate auth token", http.StatusInternalServerError)
		return
	}

	tokenStr := string(tokenType) + " " + string(signedToken)

	authCookie := &http.Cookie{
		Name:     AuthCookieName,
		Value:    tokenStr,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
	}
	if handler.isTLSEnabled {
		authCookie.Secure = true
	}
	// TODO: Add SameSite once Golang supports it
	// https://github.com/golang/go/issues/15867
	http.SetCookie(w, authCookie)

	// Deletes the oauth state cookie to avoid CSRF attacks
	http.SetCookie(w, &http.Cookie{
		Name:   cookieState.Name,
		Path:   "/",
		MaxAge: -1,
	})

	w.Header().Set(CSRFHeaderName, csrfToken)

	if oauthState.Redirect != "" && !strings.HasPrefix(oauthState.Redirect, "/") {
		hLog.Info("invalid-redirect")
		http.Error(w, "invalid redirect", http.StatusBadRequest)
		return
	}

	if oauthState.Redirect != "" {
		redirectURL, err := url.Parse(oauthState.Redirect)
		if err != nil {
			hLog.Info("invalid-redirect")
			http.Error(w, "invalid redirect", http.StatusBadRequest)
			return
		}
		queryParams := redirectURL.Query()
		queryParams.Set("csrf_token", csrfToken)
		redirectURL.RawQuery = queryParams.Encode()
		http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
		return
	}

	if oauthState.FlyLocalPort == "" {
		// Old login flow
		fmt.Fprintln(w, tokenStr)
	} else {
		encodedToken := url.QueryEscape(tokenStr)
		http.Redirect(w, r, fmt.Sprintf("http://127.0.0.1:%s/auth/callback?token=%s", oauthState.FlyLocalPort, encodedToken), http.StatusTemporaryRedirect)
	}
}
