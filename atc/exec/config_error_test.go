package exec_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("image failure text", func() {
	It("prints a numeric tag and skips credential fields", func() {
		err := exec.ImageCheckFailed("registry-image", atc.Source{
			"repository": "example/app",
			"tag":        float64(2),
			"password":   "secret-token",
		})

		Expect(err).To(MatchError(`image check failed for image "example/app" (type "registry-image", tag: 2). See the image check output for details`))
	})

	It("describes a non-registry image by its uri", func() {
		err := exec.ImageFetchFailed("git", atc.Source{
			"uri":         "https://example.com/repo.git",
			"branch":      "main",
			"private_key": "secret-key",
		})

		Expect(err).To(MatchError(`fetching image (type "git", uri: https://example.com/repo.git, branch: main) failed. See the image get output for details`))
	})
})
