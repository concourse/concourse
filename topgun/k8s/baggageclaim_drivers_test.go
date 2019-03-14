package k8s_test

import (
	"fmt"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Baggageclaim Drivers", func() {
	var (
		proxySession *gexec.Session
		releaseName  string
		namespace    string
		atcEndpoint  string
	)

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))

		if proxySession != nil {
			Wait(proxySession.Interrupt())
		}
	})

	type Case struct {
		Driver     string
		NodeImage  string
		ShouldWork bool
	}

	DescribeTable("across different node images",
		func(c Case) {
			releaseName = fmt.Sprintf("topgun-bd-%s-%s-%d",
				c.Driver, c.NodeImage, randomGenerator.Int())
			namespace = releaseName

			helmDeployTestFlags := []string{
				"--set=concourse.web.kubernetes.enabled=false",
				"--set=concourse.worker.baggageclaim.driver=" + c.Driver,
				"--set=worker.nodeSelector.nodeImage=" + c.NodeImage,
				"--set=worker.replicas=1",
			}

			deployConcourseChart(releaseName, helmDeployTestFlags...)

			if !c.ShouldWork {
				workerLogsSession := Start(nil, "kubectl", "logs",
					"--namespace="+namespace, "-lapp="+namespace+"-worker")
				<-workerLogsSession.Exited

				Expect(workerLogsSession.Out.Contents()).To(ContainSubstring("failed-to-set-up-driver"))
				return
			}

			waitAllPodsInNamespaceToBeReady(namespace)

			By("Creating the web proxy")
			proxySession, atcEndpoint = startPortForwarding(namespace, "service/"+releaseName+"-web", "8080")

			By("Logging in")
			fly.Login("test", "test", atcEndpoint)

			By("Setting and triggering a dumb pipeline")
			fly.Run("set-pipeline", "-n", "-c", "../pipelines/get-task.yml", "-p", "pipeline")
			fly.Run("trigger-job", "-j", "pipeline/simple-job")
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
