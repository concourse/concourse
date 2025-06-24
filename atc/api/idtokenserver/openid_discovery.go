package idtokenserver

import (
	"encoding/json"
	"net/http"
)

func (s *Server) OpenIDConfiguration(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("openid_configuration")

	resp := struct {
		Issuer  string `json:"issuer"`
		JWKSUri string `json:"jwks_uri"`
	}{
		// externalURL is used for the iss-claim of idtokens
		Issuer:  s.externalURL,
		JWKSUri: s.externalURL + "/.well-known/jwks.json",
	}

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		logger.Error("failed-to-encode-openid-discovery", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
