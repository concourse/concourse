package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pivotal-golang/lager"

	"golang.org/x/oauth2"
)

type OAuthCallbackHandler struct {
	logger     lager.Logger
	providers  Providers
	privateKey *rsa.PrivateKey
}

func NewOAuthCallbackHandler(
	logger lager.Logger,
	providers Providers,
	privateKey *rsa.PrivateKey,
) http.Handler {
	return &OAuthCallbackHandler{
		logger:     logger,
		providers:  providers,
		privateKey: privateKey,
	}
}

func (handler *OAuthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	provider, ok := handler.providers[r.FormValue(":provider")]
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	token, err := provider.Exchange(oauth2.NoContext, r.FormValue("code"))
	if err != nil {
		handler.logger.Error("failed-to-exchange-token", err)
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	httpClient := provider.Client(oauth2.NoContext, token)

	verified, err := provider.Verify(httpClient)
	if err != nil {
		handler.logger.Error("failed-to-verify-token", err)
		http.Error(w, "failed to verify token", http.StatusInternalServerError)
		return
	}

	if !verified {
		handler.logger.Info("verification-failed")
		http.Error(w, "verification failed", http.StatusUnauthorized)
		return
	}

	jwtToken := jwt.New(jwt.SigningMethodRS256)
	signedToken, err := jwtToken.SignedString(handler.privateKey)
	if err != nil {
		handler.logger.Error("failed-to-sign-token", err)
		http.Error(w, "failed to sign token", http.StatusInternalServerError)
		return
	}

	exp := time.Now().Add(CookieAge)
	jwtToken.Claims["exp"] = exp.Unix()

	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   "Bearer " + signedToken,
		Path:    "/",
		Expires: exp,
	})

	fmt.Fprintln(w, "ok")
}
