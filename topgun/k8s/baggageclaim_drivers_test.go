package k8s_test

import (
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Baggageclaim Drivers", func() {
	var (
		proxySession *gexec.Session
		atcEndpoint  string
	)

	AfterEach(func() {
		cleanup(releaseName, namespace, proxySession)
	})

	type Case struct {
		Driver     string
		NodeImage  string
		ShouldWork bool
	}

	DescribeTable("across different node images",
		func(c Case) {
			setReleaseNameAndNamespace("bd-" + c.Driver + "-" + c.NodeImage)

			helmDeployTestFlags := []string{
				"--set=concourse.web.kubernetes.enabled=false",
				"--set=concourse.worker.baggageclaim.driver=" + c.Driver,
				"--set=worker.nodeSelector.nodeImage=" + c.NodeImage,
				"--set=worker.replicas=1",
			}

			deployConcourseChart(releaseName, helmDeployTestFlags...)

			if !c.ShouldWork {
				Eventually(func() []byte {
					workerLogsSession := Start(nil, "kubectl", "logs",
						"--namespace="+namespace, "-lapp="+namespace+"-worker")
					<-workerLogsSession.Exited

					return workerLogsSession.Out.Contents()

				}).Should(ContainSubstring("failed-to-set-up-driver"))
				return
			}

			waitAllPodsInNamespaceToBeReady(namespace)

			By("Creating the web proxy")
			proxySession, atcEndpoint = startPortForwarding(namespace, "service/"+releaseName+"-web", "8080")

			By("Logging in")
			fly.Login("test", "test", atcEndpoint)

			By("Setting and triggering a dumb pipeline")
			fly.Run("set-pipeline", "-n", "-c", "../pipelines/get-task.yml", "-p", "some-pipeline")
			fly.Run("unpause-pipeline", "-p", "some-pipeline")
			fly.Run("trigger-job", "-w", "-j", "some-pipeline/simple-job")
		},
		Entry("with btrfs on cos", Case{
			Driver:     "btrfs",
			NodeImage:  "cos",
			ShouldWork: false,
		}),
		Entry("with btrfs on ubuntu", Case{
			Driver:     "btrfs",
			NodeImage:  "ubuntu",
			ShouldWork: true,
		}),
		Entry("with overlay on cos", Case{
			Driver:     "overlay",
			NodeImage:  "cos",
			ShouldWork: true,
		}),
		Entry("with overlay on ubuntu", Case{
			Driver:     "overlay",
			NodeImage:  "ubuntu",
			ShouldWork: true,
		}),
		Entry("with naive on cos", Case{
			Driver:     "naive",
			NodeImage:  "cos",
			ShouldWork: true,
		}),
		Entry("with naive on ubuntu", Case{
			Driver:     "naive",
			NodeImage:  "ubuntu",
			ShouldWork: true,
		}),
	)
})
