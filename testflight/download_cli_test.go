package testflight_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Download Fly CLI", func() {
	It("can download fly CLI without issue", func(ctx SpecContext) {
		flyBin, err := gexec.Build("github.com/concourse/concourse/fly")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(flyBin)

		sess := spawn(flyBin, "-t", flyTarget, "sync", "--force")
		wait(sess, false)

		Expect(sess).ToNot(gbytes.Say("warning: failed to parse Content-Length"))
		Expect(sess).To(gbytes.Say("done"))
	}, DefaultSpecTimeout)
})
