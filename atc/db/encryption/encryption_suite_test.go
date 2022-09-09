package encryption_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEncryption(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Encryption Suite")
}
