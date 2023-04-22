package k8s_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("TSA Service Node Port", func() {

	JustBeforeEach(func() {
		setReleaseNameAndNamespace("tnp")

		deployConcourseChart(releaseName,
			"--set=web.service.type=NodePort",
		)
	})

	It("deployment succeeds", func() {
		waitAndLogin(namespace, releaseName+"-web").Close()
	})

	AfterEach(func() {
		cleanupReleases()
	})

})
