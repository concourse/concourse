package event_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEvent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Event Suite")
}
