package token_test

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gopkg.in/square/go-jose.v2/jwt"
)

func TestToken(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Token Suite")
}

func n(pub *rsa.PublicKey) string {
	return encode(pub.N.Bytes())
}

func e(pub *rsa.PublicKey) string {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(pub.E))
	return encode(bytes.TrimLeft(data, "\x00"))
}

func encode(payload []byte) string {
	result := base64.URLEncoding.EncodeToString(payload)
	return strings.TrimRight(result, "=")
}

func parse(token string, key *rsa.PrivateKey, result interface{}) error {

	parsed, err := jwt.ParseSigned(token)
	if err != nil {
		return err
	}

	var claims jwt.Claims

	if err = parsed.Claims(&key.PublicKey, &claims, &result); err != nil {
		return err
	}

	return claims.Validate(jwt.Expected{Time: time.Now()})
}
