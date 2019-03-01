package k8s_test

import (
	"fmt"
	"path"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("External PostgreSQL", func() {
	var (
		releaseName   string
		pgReleaseName string
		namespace     string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-ep-%d-%d", GinkgoRandomSeed(), GinkgoParallelNode())
		namespace = releaseName
		pgReleaseName = releaseName + "-pg"

		helmDeploy(pgReleaseName,
			namespace,
			path.Join(Environment.ChartsDir, "stable/postgresql"),
			"--set=livenessProbe.initialDelaySeconds=3",
			"--set=livenessProbe.periodSeconds=3",
			"--set=persistence.enabled=false",
			"--set=postgresqlDatabase=pg-database",
			"--set=postgresqlPassword=pg-password",
			"--set=postgresqlUsername=pg-user",
			"--set=readinessProbe.initialDelaySeconds=3",
			"--set=readinessProbe.periodSeconds=3",
		)

		deployConcourseChart(releaseName,
			"--set=concourse.web.postgres.database=pg-database",
			"--set=concourse.web.postgres.host="+pgReleaseName+"-postgresql",
			"--set=concourse.worker.ephemeral=true",
			"--set=postgresql.enabled=false",
			"--set=secrets.postgresPassword=pg-password",
			"--set=secrets.postgresUser=pg-user",
			"--set=worker.replicas=0",
		)
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		helmDestroy(pgReleaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
	})

	It("gets deployed", func() {
		waitAllPodsInNamespaceToBeReady(namespace)
	})
})
