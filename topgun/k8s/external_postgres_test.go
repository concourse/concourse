package k8s_test

import (
	"path"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("External PostgreSQL", func() {
	var pgReleaseName string

	BeforeEach(func() {
		setReleaseNameAndNamespace("ep")
		pgReleaseName = releaseName + "-pg"

		helmDeploy(pgReleaseName,
			namespace,
			path.Join(Environment.HelmChartsDir, "stable/postgresql"),
			"--set=livenessProbe.initialDelaySeconds=3",
			"--set=livenessProbe.periodSeconds=3",
			"--set=persistence.enabled=false",
			"--set=auth.database=pg-database",
			"--set=auth.password=pg-password",
			"--set=auth.username=pg-user",
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

		waitAllPodsInNamespaceToBeReady(namespace)
	})

	AfterEach(func() {
		cleanupReleases()
	})

	It("can have pipelines set", func() {
		atc := endpointFactory.NewServiceEndpoint(
			namespace,
			releaseName+"-web",
			"8080",
		)
		defer atc.Close()

		By("Logging in")
		fly.Login("test", "test", "http://"+atc.Address())

		By("Setting and triggering a dumb pipeline")
		fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "pipeline")
	})
})
