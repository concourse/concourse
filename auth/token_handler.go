package auth

import (
	"bytes"
	"code.cloudfoundry.org/lager"
	"context"
	"crypto/rsa"
	"github.com/concourse/atc/db"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"time"
)

type TokenHandler struct {
	logger             lager.Logger
	authTokenGenerator AuthTokenGenerator
	expire             time.Duration
	csrfTokenGenerator CSRFTokenGenerator
	teamFactory        db.TeamFactory
	providerFactory    ProviderFactory
}

func NewTokenHandler(
	logger lager.Logger,
	providerFactory ProviderFactory,
	privateKey *rsa.PrivateKey,
	teamFactory db.TeamFactory,
	expire time.Duration,
) http.Handler {
	return &TokenHandler{
		logger:             logger,
		authTokenGenerator: NewAuthTokenGenerator(privateKey),
		expire:             expire,
		csrfTokenGenerator: NewCSRFTokenGenerator(),
		teamFactory:        teamFactory,
		providerFactory:    providerFactory,
	}
}

func (handler *TokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	hLog := handler.logger.Session("token")
	providerName := r.FormValue(":provider")
	teamName := r.FormValue("team_name")

	token := StreamToString(r.Body)

	if token == "" {
		handler.logger.Info("failed-to-find-token-in-body")
		http.Error(w, "Token is not present in body", http.StatusBadRequest)
		return
	}

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

	// crete new client that supports a token authentication
	ctx := context.Background()

	tc := provider.Client(ctx, &oauth2.Token{AccessToken: token})

	verified, err := provider.Verify(handler.logger, tc)
	if err != nil {
		handler.logger.Error("error-while-verifying-user", err)
		http.Error(w, "error durring verification", http.StatusInternalServerError)
		return
	}
	if !verified {
		handler.logger.Info("failed-to-verify-user", lager.Data{
			"teamName": teamName,
		})
		http.Error(w, "not verified", http.StatusUnauthorized)
		return
	}

	// generate token
	exp := time.Now().Add(handler.expire)

	csrfToken, err := handler.csrfTokenGenerator.GenerateToken()
	if err != nil {
		handler.logger.Error("generate-csrf-token", err)
		http.Error(w, "failed to generate csrf token", http.StatusInternalServerError)
		return
	}

	tokenType, signedToken, err := handler.authTokenGenerator.GenerateToken(exp, team.Name(), team.Admin(), csrfToken)
	if err != nil {
		handler.logger.Error("failed-to-sign-token", err)
		http.Error(w, "failed to generate auth token", http.StatusInternalServerError)
		return
	}

	tokenStr := string(tokenType) + " " + string(signedToken)

	io.WriteString(w, tokenStr)
}

func StreamToString(stream io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	return buf.String()
}
