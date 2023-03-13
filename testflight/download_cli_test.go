package testflight_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Download Fly Cli", func() {
	var beforeFly string

	BeforeEach(func() {
		beforeFly = config.FlyBin

		var err error
		config.FlyBin, err = gexec.Build("github.com/concourse/concourse/fly")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(config.FlyBin)
		Expect(err).ToNot(HaveOccurred())

		config.FlyBin = beforeFly
	})

	It("can download fly cli without issue", func() {
		watch := fly("sync", "--force")
		Expect(watch).ToNot(gbytes.Say("warning: failed to parse Content-Length"))
		Expect(watch).To(gbytes.Say("done"))
	})
})
