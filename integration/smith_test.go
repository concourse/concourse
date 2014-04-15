package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/vito/cmdtest/matchers"

	"github.com/vito/cmdtest"
)

func tarFiles(path string) string {
	output, err := exec.Command("tar", "tvf", path).Output()
	Expect(err).ToNot(HaveOccurred())

	return string(output)
}

var _ = Describe("Smith CLI", func() {
	It("tars up the current directory", func() {
		smithPath, err := cmdtest.Build("github.com/room101-ci/smith")
		Expect(err).ToNot(HaveOccurred())

		smithCmd := exec.Command(smithPath)
		smithSession, err := cmdtest.Start(smithCmd)
		Expect(err).ToNot(HaveOccurred())
		Expect(smithSession).To(ExitWith(0))

		Expect(tarFiles("dir.tgz")).To(ContainSubstring("smith_test.go"))
	})
})
