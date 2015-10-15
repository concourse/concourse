package auth

import "net/http"

type NoopValidator struct{}

func (NoopValidator) IsAuthenticated(*http.Request) bool { return true }
