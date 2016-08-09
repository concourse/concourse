package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/concourse/atc/db"

	"code.cloudfoundry.org/lager"
)

const OAuthStateCookie = "_concourse_oauth_state"

type OAuthState struct {
	Redirect string `json:"redirect"`
	TeamName string `json:"team_name"`
}

type OAuthBeginHandler struct {
	logger          lager.Logger
	providerFactory ProviderFactory
	privateKey      *rsa.PrivateKey
	teamDBFactory   db.TeamDBFactory
}

func NewOAuthBeginHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	privateKey *rsa.PrivateKey,
	teamDBFactory db.TeamDBFactory,
) http.Handler {
	return &OAuthBeginHandler{
		logger:          logger,
		providerFactory: providerFactory,
		privateKey:      privateKey,
		teamDBFactory:   teamDBFactory,
	}
}

func (handler *OAuthBeginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hLog := handler.logger.Session("oauth-begin")
	providerName := r.FormValue(":provider")
	teamName := r.FormValue("team_name")

	teamDB := handler.teamDBFactory.GetTeamDB(teamName)
	team, found, err := teamDB.GetTeam()
	if err != nil {
		hLog.Error("failed-to-get-team", err, lager.Data{
			"teamName": teamName,
		})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		hLog.Info("failed-to-find-team", lager.Data{
			"teamName": teamName,
		})
		w.WriteHeader(http.StatusNotFound)
		return
	}

	providers, err := handler.providerFactory.GetProviders(team)
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
		handler.logger.Info("unknown-provider", lager.Data{
			"provider": providerName,
		})

		w.WriteHeader(http.StatusNotFound)
		return
	}

	oauthState, err := json.Marshal(OAuthState{
		Redirect: r.FormValue("redirect"),
		TeamName: teamName,
	})
	if err != nil {
		handler.logger.Error("failed-to-marshal-state", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	encodedState := base64.RawURLEncoding.EncodeToString(oauthState)

	authCodeURL := provider.AuthCodeURL(encodedState)

	http.SetCookie(w, &http.Cookie{
		Name:    OAuthStateCookie,
		Value:   encodedState,
		Path:    "/",
		Expires: time.Now().Add(CookieAge),
	})

	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}
