package auth

import "net/http"

type ValidatorBasket []Validator

func (vb ValidatorBasket) IsAuthenticated(r *http.Request) bool {
	for _, validator := range vb {
		if validator.IsAuthenticated(r) {
			return true
		}
	}

	return false
}
