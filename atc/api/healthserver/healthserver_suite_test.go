package healthserver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHealthserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Healthserver Suite")
}
