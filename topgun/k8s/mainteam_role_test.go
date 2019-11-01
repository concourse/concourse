package k8s_test

import (
	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main team role config", func() {
	var (
		atc Endpoint

		helmDeployTestFlags []string
		username, password  = "test-viewer", "test-viewer"
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("mt")
		Run(nil, "kubectl", "create", "namespace", namespace)
	})

	JustBeforeEach(func() {
		deployConcourseChart(releaseName, helmDeployTestFlags...)

		waitAllPodsInNamespaceToBeReady(namespace)

		atc = endpointFactory.NewServiceEndpoint(
			namespace,
			releaseName+"-web",
			"8080",
		)

		fly.Login(username, password, "http://"+atc.Address())

	})

	AfterEach(func() {
		atc.Close()
		cleanup(releaseName, namespace)
	})

	Context("Adding team role config yaml to web", func() {
		BeforeEach(func() {
			helmDeployTestFlags = []string{
				`--set=worker.enabled=false`,
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
