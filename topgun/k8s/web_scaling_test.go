package k8s_test

import (
	"strconv"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Scaling web instances", func() {

	BeforeEach(func() {
		setReleaseNameAndNamespace("swi")
	})

	AfterEach(func() {
		cleanup(releaseName, namespace)
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
