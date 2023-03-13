package configvalidate_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigvalidate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configvalidate Suite")
}
