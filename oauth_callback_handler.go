package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"

	"net/url"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

type OAuthCallbackHandler struct {
	logger          lager.Logger
	providerFactory ProviderFactory
	privateKey      *rsa.PrivateKey
	tokenGenerator  TokenGenerator
	teamDBFactory   db.TeamDBFactory
	expire          time.Duration
}

func NewOAuthCallbackHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	privateKey *rsa.PrivateKey,
	teamDBFactory db.TeamDBFactory,
	expire time.Duration,
) http.Handler {
	return &OAuthCallbackHandler{
		logger:          logger,
		providerFactory: providerFactory,
		privateKey:      privateKey,
		tokenGenerator:  NewTokenGenerator(privateKey),
		teamDBFactory:   teamDBFactory,
		expire:          expire,
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

	teamName := oauthState.TeamName
	teamDB := handler.teamDBFactory.GetTeamDB(teamName)
	team, found, err := teamDB.GetTeam()
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

	token, err := provider.Exchange(ctx, r.FormValue("code"))
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

	// Deletes the oauth state cookie to avoid CSRF attacks
	http.SetCookie(w, &http.Cookie{
		Name:   cookieState.Name,
		Path:   "/",
		MaxAge: -1,
	})

	if oauthState.Redirect != "" {
		http.Redirect(w, r, oauthState.Redirect, http.StatusTemporaryRedirect)
		return
	}

	if oauthState.FlyLocalPort == "" {
		// Old login flow
		fmt.Fprintln(w, tokenStr)
	} else {
		encodedToken := url.QueryEscape(tokenStr)
		http.Redirect(w, r, fmt.Sprintf("http://127.0.0.1:%s/oauth/callback?token=%s", oauthState.FlyLocalPort, encodedToken), http.StatusTemporaryRedirect)
	}
}
