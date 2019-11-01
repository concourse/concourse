package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Download Fly Cli", func() {
	It("can download fly cli without issue", func() {
		watch := fly("sync", "--force")
		Expect(watch).ToNot(gbytes.Say("warning: failed to parse Content-Length"))
		Expect(watch).To(gbytes.Say("done"))
	})
})
