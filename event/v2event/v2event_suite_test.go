package v2event_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestV2event(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "V2 Event Suite")
}
