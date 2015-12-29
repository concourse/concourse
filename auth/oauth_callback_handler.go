package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"

	"golang.org/x/oauth2"
)

type OAuthCallbackHandler struct {
	logger          lager.Logger
	providerFactory ProviderFactory
	privateKey      *rsa.PrivateKey
	tokenGenerator  TokenGenerator
	db              AuthDB
}

func NewOAuthCallbackHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	privateKey *rsa.PrivateKey,
	db AuthDB,
) http.Handler {
	return &OAuthCallbackHandler{
		logger:          logger,
		providerFactory: providerFactory,
		privateKey:      privateKey,
		tokenGenerator:  NewTokenGenerator(privateKey),
		db:              db,
	}
}

func (handler *OAuthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hLog := handler.logger.Session("callback")
	providerName := r.FormValue(":provider")
	teamName := atc.DefaultTeamName

	team, found, err := handler.db.GetTeamByName(atc.DefaultTeamName)
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

	providers, err := handler.providerFactory.GetProviders(teamName)
	if err != nil {
		handler.logger.Error("unknown-provider", err, lager.Data{
			"provider": providerName,
			"teamName": teamName,
		})

		w.WriteHeader(http.StatusNotFound)
		return
	}

	provider, found := providers[providerName]
	if !found {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	paramState := r.FormValue("state")

	cookieState, err := r.Cookie(OAuthStateCookie)
	if err != nil {
		hLog.Info("no-state-cookie", lager.Data{
			"error": err.Error(),
		})
		http.Error(w, "state cookie not set", http.StatusUnauthorized)
		return
	}

	if cookieState.Value != paramState {
		hLog.Info("state-cookie-mismatch", lager.Data{
			"param-state":  paramState,
			"cookie-state": cookieState.Value,
		})

		http.Error(w, "state cookie does not match param", http.StatusUnauthorized)
		return
	}

	stateJSON, err := base64.RawURLEncoding.DecodeString(r.FormValue("state"))
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

	token, err := provider.Exchange(oauth2.NoContext, r.FormValue("code"))
	if err != nil {
		hLog.Error("failed-to-exchange-token", err)
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	httpClient := provider.Client(oauth2.NoContext, token)

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

	exp := time.Now().Add(CookieAge)

	tokenType, signedToken, err := handler.tokenGenerator.GenerateToken(exp, team.Name, team.ID, team.Admin)
	if err != nil {
		hLog.Error("failed-to-sign-token", err)
		http.Error(w, "failed to sign token", http.StatusInternalServerError)
		return
	}

	tokenStr := string(tokenType) + " " + string(signedToken)

	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   tokenStr,
		Path:    "/",
		Expires: exp,
	})

	if oauthState.Redirect != "" {
		http.Redirect(w, r, oauthState.Redirect, http.StatusTemporaryRedirect)
		return
	}

	fmt.Fprintln(w, tokenStr)
}
