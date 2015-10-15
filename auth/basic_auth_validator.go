package auth

import "net/http"

type BasicAuthValidator struct {
	Username string
	Password string
}

func (validator BasicAuthValidator) IsAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")

	username, password, err := extractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	return validator.correctCredentials(username, password)
}

func (validator BasicAuthValidator) correctCredentials(username string, password string) bool {
	return validator.Username == username && validator.Password == password
}
