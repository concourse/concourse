package api_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Suite")
}
