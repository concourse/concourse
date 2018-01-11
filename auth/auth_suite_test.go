package auth_test

import (
	"encoding/base64"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Suite")
}

func header(stringList ...string) string {
	credentials := []byte(strings.Join(stringList, ":"))
	return "Basic " + base64.StdEncoding.EncodeToString(credentials)
}
