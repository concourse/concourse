package auth

import (
	"encoding/base64"
	"errors"
	"strings"
)

var ErrUnparsableHeader = errors.New("cannot parse 'Authorization' header")

func extractUsernameAndPassword(authorizationHeader string) (string, string, error) {
	if !strings.HasPrefix(strings.ToUpper(authorizationHeader), "BASIC ") {
		return "", "", ErrUnparsableHeader
	}

	encodedCredentials := authorizationHeader[6:]
	credentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return "", "", ErrUnparsableHeader
	}

	parts := strings.Split(string(credentials), ":")
	if len(parts) != 2 {
		return "", "", ErrUnparsableHeader
	}

	return parts[0], parts[1], nil
}
