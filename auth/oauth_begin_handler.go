package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/concourse/atc/db"

	"strconv"

	"code.cloudfoundry.org/lager"
	//"github.com/dghubble/oauth1"
	//"golang.org/x/oauth2"
	"github.com/concourse/atc/auth/provider"
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
	teamFactory     db.TeamFactory
	expire          time.Duration
	isTLSEnabled    bool
	versionHandler  oauthBeginHandler
}

func NewOAuthBeginHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	privateKey *rsa.PrivateKey,
	teamFactory db.TeamFactory,
	expire time.Duration,
	isTLSEnabled bool,
	versionHandler oauthBeginHandler,
) http.Handler {
	return &OAuthBeginHandler{
		logger:          logger,
		providerFactory: providerFactory,
		privateKey:      privateKey,
		teamFactory:     teamFactory,
		expire:          expire,
		isTLSEnabled:    isTLSEnabled,
		versionHandler:  versionHandler,
	}
}

func (handler *OAuthBeginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hLog := handler.logger.Session("oauth-begin")
	providerName := r.FormValue(":provider")
	teamName := r.FormValue("team_name")

	team, found, err := handler.teamFactory.FindTeam(teamName)

	if err != nil {
		hLog.Error("failed-to-get-team", err, lager.Data{
			"teamName": teamName,
			"provider": providerName,
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
	_, err = strconv.Atoi(r.FormValue("fly_local_port"))
	if r.FormValue("fly_local_port") != "" && err != nil {
		handler.logger.Error("failed-to-convert-port-to-integer", err)
		w.WriteHeader(http.StatusInternalServerError)
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

	authCodeURL := handler.versionHandler.AuthCodeURL(oauthState, provider, handler, w)

	http.Redirect(w, r, authCodeURL, http.StatusTemporaryRedirect)
}

type oauthBeginHandler interface {
	AuthCodeURL(oauthState []byte, provider provider.Provider, handler *OAuthBeginHandler, w http.ResponseWriter) string
}
type oauthBeginHandlerV1 struct{}
type oauthBeginHandlerV2 struct{}

func (oauthBeginHandlerV1) AuthCodeURL(oauthState []byte, provider provider.Provider, handler *OAuthBeginHandler, w http.ResponseWriter) string {
	return provider.AuthCodeURL("")
}

func (oauthBeginHandlerV2) AuthCodeURL(oauthState []byte, provider provider.Provider, handler *OAuthBeginHandler, w http.ResponseWriter) string {
	encodedState := base64.RawURLEncoding.EncodeToString(oauthState)

	authCodeURL := provider.AuthCodeURL(encodedState)

	authCookie := &http.Cookie{
		Name:     OAuthStateCookie,
		Value:    encodedState,
		Path:     "/",
		Expires:  time.Now().Add(handler.expire),
		HttpOnly: true,
	}
	if handler.isTLSEnabled {
		authCookie.Secure = true
	}
	// TODO: Add SameSite once Golang supports it
	// https://github.com/golang/go/issues/15867
	http.SetCookie(w, authCookie)
	return authCodeURL
}
