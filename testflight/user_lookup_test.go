package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("user lookup", func() {
	Context("when no username is specified and the image has no /etc/passwd file", func() {
		It("runs as root", func(ctx SpecContext) {
			// clearlinux doesn't have an /etc/passwd file
			session := fly("execute", "-c", "fixtures/clearlinux.yml")
			Expect(session.Out).To(gbytes.Say("running as root"))
		}, DefaultSpecTimeout)
	})
})
