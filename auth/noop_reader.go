package auth

import "net/http"

type NoopReader struct{}

func (NoopReader) GetTeam(r *http.Request) (string, int, bool, bool) { return "", 0, false, false }
