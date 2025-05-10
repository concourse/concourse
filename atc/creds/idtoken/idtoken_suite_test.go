package idtoken_test

import (
	"testing"

	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/go-jose/go-jose/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var rsaJWK *jose.JSONWebKey

func TestIDToken(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IDToken Suite")
}

var _ = BeforeSuite(func() {
	var err error
	// generate this in beforeSuite because it is expensive and we don't need to re-run in for every test
	rsaJWK, err = idtoken.GenerateNewRSAKey()
	Expect(err).ToNot(HaveOccurred())
})
