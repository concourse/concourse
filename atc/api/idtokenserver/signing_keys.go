package idtokenserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-jose/go-jose/v3"
)

func (s *Server) SigningKeys(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("signing_keys")

	currentKeys, err := s.dbSigningKeyFactory.GetAllKeys()
	if err != nil {
		logger.Error("failed-to-get-signing-keys", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	publicKeys := jose.JSONWebKeySet{
		Keys: make([]jose.JSONWebKey, len(currentKeys)),
	}
	for i, key := range currentKeys {
		privateKey := key.JWK()
		// This step is ABSOLUTELY CRITICAL! Only publish the public part of the key!
		publicKey := (&privateKey).Public()
		publicKeys.Keys[i] = publicKey
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(publicKeys)
	if err != nil {
		logger.Error("failed-to-encode-jwks", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
