package authredirect_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuthredirect(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Authredirect Suite")
}
