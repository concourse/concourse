package atc

import (
	"encoding/json"

	"golang.org/x/crypto/bcrypt"
)

type Team struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	BasicAuth *BasicAuth `json:"basic_auth,omitempty"`

	Auth map[string]*json.RawMessage `json:"auth,omitempty"`
}

type BasicAuth struct {
	BasicAuthUsername string `json:"basic_auth_username,omitempty"`
	BasicAuthPassword string `json:"basic_auth_password,omitempty"`
}

func (auth *BasicAuth) EncryptedJSON() (string, error) {
	var result *BasicAuth
	if auth != nil && auth.BasicAuthUsername != "" && auth.BasicAuthPassword != "" {
		encryptedPw, err := bcrypt.GenerateFromPassword([]byte(auth.BasicAuthPassword), 4)
		if err != nil {
			return "", err
		}
		result = &BasicAuth{
			BasicAuthPassword: string(encryptedPw),
			BasicAuthUsername: auth.BasicAuthUsername,
		}
	}

	json, err := json.Marshal(result)
	return string(json), err
}
