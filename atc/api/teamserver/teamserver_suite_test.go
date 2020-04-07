package teamserver_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTeamserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Teamserver Suite")
}
