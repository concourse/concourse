package accessor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	ErrVerificationNoToken          = errors.New("token not provided")
	ErrVerificationInvalidToken     = errors.New("token provided is invalid")
	ErrVerificationFetchFailed      = errors.New("failed to fetch public key")
	ErrVerificationInvalidSignature = errors.New("token has invalid signature")
	ErrVerificationTokenExpired     = errors.New("token is expired")
	ErrVerificationInvalidAudience  = errors.New("token has invalid audience")
	ErrVerificationFailed           = errors.New("token verification failed")
)

func NewVerifier(httpClient *http.Client, keySetURL *url.URL, audience []string) *verifier {
	return &verifier{
		httpClient: httpClient,
		keySetURL:  keySetURL,
		audience:   audience,
	}
}

type verifier struct {
	sync.Mutex
	httpClient  *http.Client
	keySetURL   *url.URL
	keySet      jose.JSONWebKeySet
	lastRefresh time.Time
	audience    []string
}

func (v *verifier) Verify(r *http.Request) (map[string]interface{}, error) {

	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, ErrVerificationNoToken
	}

	parts := strings.Split(header, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, ErrVerificationInvalidToken
	}

	return v.verify(parts[1])
}

func (v *verifier) verify(raw string) (map[string]interface{}, error) {

	token, err := jwt.ParseSigned(raw)
	if err != nil {
		return nil, ErrVerificationInvalidToken
	}

	var claims jwt.Claims
	var data map[string]interface{}

	err = token.Claims(&v.keySet, &claims, &data)
	if err != nil {

		if err = v.refreshKeySet(); err != nil {
			return nil, err
		}

		err = token.Claims(&v.keySet, &claims, &data)
		if err != nil {
			return nil, ErrVerificationInvalidSignature
		}
	}

	err = claims.Validate(jwt.Expected{Time: time.Now()})
	if err != nil {
		return nil, ErrVerificationTokenExpired
	}

	for _, aud := range v.audience {
		if claims.Audience.Contains(aud) {
			return data, nil
		}
	}

	return nil, ErrVerificationInvalidAudience
}

func (v *verifier) refreshKeySet() error {
	v.Lock()
	defer v.Unlock()

	if time.Since(v.lastRefresh) < time.Minute {
		return nil
	}

	key, err := v.fetchKeySet()
	if err != nil {
		return ErrVerificationFetchFailed
	}

	v.keySet = key
	v.lastRefresh = time.Now()

	return nil
}

func (v *verifier) fetchKeySet() (jose.JSONWebKeySet, error) {
	var keys jose.JSONWebKeySet

	resp, err := v.httpClient.Get(v.keySetURL.String())
	if err != nil {
		return keys, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return keys, fmt.Errorf("error: %v", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return keys, err
	}

	return keys, nil
}
