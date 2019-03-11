package k8s_test

import (
	"fmt"
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main team role config", func() {
	var (
		proxySession        *gexec.Session
		releaseName         string
		namespace           string
		atcEndpoint  		string
		helmDeployTestFlags []string
		username     = "test-viewer"
		password     = "test-viewer"
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-tr-%d-%d", GinkgoRandomSeed(), GinkgoParallelNode())
		namespace = releaseName
		Run(nil, "kubectl", "create", "namespace", namespace)
	})

	JustBeforeEach(func() {
		deployConcourseChart(releaseName, helmDeployTestFlags...)

		waitAllPodsInNamespaceToBeReady(namespace)

		pods := getPods(namespace, "--selector=app="+releaseName+"-worker")
		Expect(pods).To(HaveLen(1))

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(namespace, releaseName+"-web", "8080")

		By("Logging in")
		fly.Login(username, password, atcEndpoint)

	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
		Wait(proxySession.Interrupt())
	})

	Context("Adding team role config yaml to web", func() {
		BeforeEach(func() {
			helmDeployTestFlags = []string{
				`--set=worker.replicas=1`,
				`--set=web.additionalVolumes[0].name=team-role-config`,
				`--set=web.additionalVolumes[0].configMap.name=team-role-config`,
				`--set=web.additionalVolumeMounts[0].name=team-role-config`,
				`--set=web.additionalVolumeMounts[0].mountPath=/foo`,
				`--set=concourse.web.auth.mainTeam.config=/foo/team-role-config.yml`,
				`--set=secrets.localUsers=test-viewer:test-viewer`,
			}

			configMapCreationArgs := []string{
				"create",
				"configmap",
				"team-role-config",
				"--namespace=" + namespace,
				`--from-literal=team-role-config.yml=
roles:
 - name: viewer
   local:
     users: [ "test-viewer" ]
 `,
			}

			Run(nil, "kubectl", configMapCreationArgs...)

		})

		It("returns the correct user role", func() {
			userRole := fly.GetUserRole("main")
			Expect(userRole).To(HaveLen(1))
			Expect(userRole[0]).To(Equal("viewer"))
		})
	})

})
