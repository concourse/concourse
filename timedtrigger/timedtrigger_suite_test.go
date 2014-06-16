package timedtrigger_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTimedtrigger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Timedtrigger Suite")
}
