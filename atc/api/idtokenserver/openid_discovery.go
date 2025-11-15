package idtokenserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-jose/go-jose/v4"
)

func (s *Server) OpenIDConfiguration(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("openid_configuration")

	resp := struct {
		Issuer                           string   `json:"issuer"`
		JWKSUri                          string   `json:"jwks_uri"`
		ClaimsSupported                  []string `json:"claims_supported"`
		ResponseTypesSupported           []string `json:"response_types_supported"`
		IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
		SubjectTypesSupported            []string `json:"subject_types_supported"`
	}{
		// Use oidcIssuer (which falls back to externalURL if not set)
		Issuer:                           s.oidcIssuer,
		JWKSUri:                          s.oidcIssuer + "/.well-known/jwks.json",
		ClaimsSupported:                  []string{"aud", "iat", "iss", "sub"},
		ResponseTypesSupported:           []string{"idtoken"},
		IDTokenSigningAlgValuesSupported: []string{string(jose.RS256), string(jose.ES256)},
		SubjectTypesSupported:            []string{"public"},
	}

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		logger.Error("failed-to-encode-openid-discovery", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
