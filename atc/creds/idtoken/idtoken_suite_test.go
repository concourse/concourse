package idtoken_test

import (
	"testing"

	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"
	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var rsaJWK *jose.JSONWebKey
var ecJWK *jose.JSONWebKey

func TestIDToken(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IDToken Suite")
}

var _ = BeforeSuite(func() {
	var err error
	// generate this in beforeSuite because it is expensive and we don't need to re-run in for every test
	rsaJWK, err = idtoken.GenerateNewKey(db.SigningKeyTypeRSA)
	Expect(err).ToNot(HaveOccurred())

	ecJWK, err = idtoken.GenerateNewKey(db.SigningKeyTypeEC)
	Expect(err).ToNot(HaveOccurred())
})
