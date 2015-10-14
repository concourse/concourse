package auth

import "net/http"

type ValidatorBasket struct {
	Validators []Validator

	Rejector
}

func (vb ValidatorBasket) IsAuthenticated(r *http.Request) bool {
	for _, validator := range vb.Validators {
		if validator.IsAuthenticated(r) {
			return true
		}
	}

	return false
}
