package token

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"gopkg.in/square/go-jose.v2/jwt"
)

//go:generate counterfeiter . Generator

type Generator interface {
	GenerateAccessToken() (string, error)
}

//go:generate counterfeiter . ClaimsParser

type ClaimsParser interface {
	ParseClaims(idToken string) (db.Claims, error)
}

func StoreAccessToken(logger lager.Logger, handler http.Handler, generator Generator, claimsParser ClaimsParser, accessTokenFactory db.AccessTokenFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sky/issuer/token" {
			handler.ServeHTTP(w, r)
			return
		}
		logger := logger.Session("token-request")
		logger.Debug("start")
		defer logger.Debug("end")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)

		var body io.Reader
		defer func() {
			copyResponseHeaders(w, rec.Result())
			if body != nil {
				io.Copy(w, body)
			}
		}()
		if rec.Code < 200 || rec.Code > 299 {
			body = rec.Body
			return
		}
		var resp struct {
			AccessToken  string `json:"access_token"`
			TokenType    string `json:"token_type"`
			ExpiresIn    int    `json:"expires_in"`
			RefreshToken string `json:"refresh_token,omitempty"`
			IDToken      string `json:"id_token"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		if err != nil {
			logger.Error("unmarshal-response-from-dex", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		claims, err := claimsParser.ParseClaims(resp.IDToken)
		if err != nil {
			logger.Error("parse-id-token", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp.AccessToken, err = generator.GenerateAccessToken()
		if err != nil {
			logger.Error("generate-access-token", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = accessTokenFactory.CreateAccessToken(resp.AccessToken, claims)
		if err != nil {
			logger.Error("create-access-token-in-db", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		newResp, err := json.Marshal(resp)
		if err != nil {
			logger.Error("marshal-new-response", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body = bytes.NewReader(newResp)
	})
}

func copyResponseHeaders(w http.ResponseWriter, res *http.Response) {
	for k, v := range res.Header {
		k = http.CanonicalHeaderKey(k)
		if k != "Content-Length" {
			w.Header()[k] = v
		}
	}
	w.WriteHeader(res.StatusCode)
}

func NewClaimsParser() ClaimsParser {
	return claimsParserNoVerify{}
}

type claimsParserNoVerify struct {
}

func (claimsParserNoVerify) ParseClaims(idToken string) (db.Claims, error) {
	token, err := jwt.ParseSigned(idToken)
	if err != nil {
		return db.Claims{}, err
	}

	var claims db.Claims
	err = token.UnsafeClaimsWithoutVerification(&claims)
	if err != nil {
		return db.Claims{}, err
	}
	return claims, nil
}

func NewGenerator() Generator {
	return randomTokenGenerator{}
}

type randomTokenGenerator struct {
}

func (randomTokenGenerator) GenerateAccessToken() (string, error) {
	b := [20]byte{}
	_, err := rand.Read(b[:])
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	_, err = base64.NewEncoder(base64.StdEncoding, buf).Write(b[:])
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
