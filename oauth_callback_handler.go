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

type Verifier interface {
	Verify(*http.Client) (bool, error)
}

type OAuthCallbackHandler struct {
	Config     *oauth2.Config
	Logger     lager.Logger
	Verifier   Verifier
	PrivateKey *rsa.PrivateKey
}

func (handler *OAuthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler.Config == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	token, err := handler.Config.Exchange(oauth2.NoContext, r.FormValue("code"))
	if err != nil {
		handler.Logger.Error("failed-to-exchange-token", err)
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	httpClient := handler.Config.Client(oauth2.NoContext, token)

	verified, err := handler.Verifier.Verify(httpClient)
	if err != nil {
		handler.Logger.Error("failed-to-verify-token", err)
		http.Error(w, "failed to verify token", http.StatusInternalServerError)
		return
	}

	if !verified {
		handler.Logger.Info("verification-failed")
		http.Error(w, "verification failed", http.StatusUnauthorized)
		return
	}

	jwtToken := jwt.New(jwt.SigningMethodRS256)
	signedToken, err := jwtToken.SignedString(handler.PrivateKey)
	if err != nil {
		handler.Logger.Error("failed-to-sign-token", err)
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
