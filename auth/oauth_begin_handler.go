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
	Redirect     string `json:"redirect"`
	TeamName     string `json:"team_name"`
	FlyLocalPort string `json:"fly_local_port"`
}

type OAuthBeginHandler struct {
	logger          lager.Logger
	providerFactory ProviderFactory
	privateKey      *rsa.PrivateKey
	teamDBFactory   db.TeamDBFactory
	expire          time.Duration
}

func NewOAuthBeginHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	privateKey *rsa.PrivateKey,
	teamDBFactory db.TeamDBFactory,
	expire time.Duration,
) http.Handler {
	return &OAuthBeginHandler{
		logger:          logger,
		providerFactory: providerFactory,
		privateKey:      privateKey,
		teamDBFactory:   teamDBFactory,
		expire:          expire,
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
		handler.logger.Info("team-does-not-have-auth-provider", lager.Data{
			"provider": providerName,
		})

		w.WriteHeader(http.StatusNotFound)
		return
	}

	oauthState, err := json.Marshal(OAuthState{
		Redirect:     r.FormValue("redirect"),
		TeamName:     teamName,
		FlyLocalPort: r.FormValue("fly_local_port"),
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
		Expires: time.Now().Add(handler.expire),
	})
	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}
