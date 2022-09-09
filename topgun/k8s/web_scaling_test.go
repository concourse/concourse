package k8s_test

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Scaling web instances", func() {

	BeforeEach(func() {
		setReleaseNameAndNamespace("swi")
	})

	AfterEach(func() {
		cleanupReleases()
	})

	It("succeeds", func() {
		successfullyDeploysConcourse(1)
		successfullyDeploysConcourse(0)
		successfullyDeploysConcourse(2)
	})
})

func successfullyDeploysConcourse(webReplicas int) {
	deployConcourseChart(releaseName,
		"--set=web.replicas="+strconv.Itoa(webReplicas),
		"--set=worker.replicas=1",
	)

	waitAndLogin(namespace, releaseName+"-web").Close()
}
