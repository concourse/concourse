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
	hLog := handler.logger.Session("callback")

	provider, found := handler.providers[r.FormValue(":provider")]
	if !found {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	stateToken, err := jwt.Parse(r.FormValue("state"), keyFunc(handler.privateKey))
	if err != nil {
		hLog.Info("failed-to-verify-state", lager.Data{
			"error": err.Error(),
		})
		http.Error(w, "cannot verify state", http.StatusUnauthorized)
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

	jwtToken := jwt.New(SigningMethod)

	exp := time.Now().Add(CookieAge)
	jwtToken.Claims["exp"] = exp.Unix()

	signedToken, err := jwtToken.SignedString(handler.privateKey)
	if err != nil {
		hLog.Error("failed-to-sign-token", err)
		http.Error(w, "failed to sign token", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   "Bearer " + signedToken,
		Path:    "/",
		Expires: exp,
	})

	redirectPath, ok := stateToken.Claims["redirect"].(string)
	if ok && redirectPath != "" {
		http.Redirect(w, r, redirectPath, http.StatusTemporaryRedirect)
		return
	}

	fmt.Fprintln(w, "ok")
}
