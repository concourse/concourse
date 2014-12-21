package v1event_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestV1event(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "V1 Event Suite")
}
