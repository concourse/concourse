package k8s_test

import (
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("team external workers through separate deployments", func() {

	var (
		proxySession *gexec.Session
		atcEndpoint  string
		workerKey    string
	)

	JustBeforeEach(func() {
		setReleaseNameAndNamespace("xw")

		By("creating a web only deployment in one namespace")
		helmArgs := []string{
			"--set=worker.enabled=false",

			"--set=secrets.teamAuthorizedKeys[0].team=main",
			"--set=secrets.teamAuthorizedKeys[0].key=" + workerKey,

			"--set=web.env[0].name=CONCOURSE_TSA_AUTHORIZED_KEYS",
			"--set=web.env[0].value=",
		}
		deployConcourseChart(releaseName+"-web", helmArgs...)

		By("creating a worker only deployment in another namespace")
		helmArgs = []string{
			"--set=postgresql.enabled=false",
			"--set=web.enabled=false",
			"--set=concourse.worker.team=main",

			"--set=worker.replicas=1",

			"--set=concourse.worker.tsa.host=" + releaseName + "-web-web." + releaseName + "-web.svc.cluster.local",
		}
		deployConcourseChart(releaseName+"-worker", helmArgs...)

		waitAllPodsInNamespaceToBeReady(namespace + "-worker")
		waitAllPodsInNamespaceToBeReady(namespace + "-web")

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(namespace+"-web", "service/"+releaseName+"-web-web", "8080")

		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

	})

	Context("web with correct public key to the worker", func() {
		BeforeEach(func() {
			workerKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC496FSYFcBAKgDtMsBAJiF/6/NxlXKP5UZecyEsedYuTt1GOgJTwaA1qZ1LmHsbfLDE68oDdiM4uvxfI4wtLhz57w3u0jOUxZ2JeF7SVwEf1nVqLn4Gh/f8GUNQGSyIp1zUD5Bx9fq0PAyQ47mt7Ufi84rcf8LKl7nzAIHTcdg2BvTkQN9bUGPaq/Pb1W2bKPAQy4OzXTSIyrAJ89TH2jFeaZfyxQFGbD9jVHH/yl0oiMrDeaRYgccE5II+KY7WoLjsBry/9Qf2ERELKTK4UeIGIqWci9lab1ti+GxFPPiC3krNFjo4jShV4eUs4cNIrjwNrxVaKPXmU6o7Y3Hpayx Concourse"
		})
		It("worker registers with team main", func() {
			By("waiting for a running worker")
			Eventually(func() []Worker {
				return getRunningWorkers(fly.GetWorkers())
			}, 2*time.Minute, 10*time.Second).
				ShouldNot(HaveLen(0))
			worker := getRunningWorkers(fly.GetWorkers())
			Expect(worker).To(HaveLen(1))
			Expect(worker[0].Team).To(Equal("main"))
		})
	})
	Context("web with invalid public key to the worker", func() {
		BeforeEach(func() {
			workerKey = "ssh-rsa 1234ABCD Concourse"
		})
		It("worker doesn't registers with team main", func() {
			Consistently(func() []Worker {
				return getRunningWorkers(fly.GetWorkers())
			}, 1*time.Minute, 10*time.Second).
				Should(HaveLen(0))
		})
	})

	AfterEach(func() {
		cleanup(releaseName+"-web", namespace+"-web", proxySession)
		cleanup(releaseName+"-worker", namespace+"-worker", nil)
	})

})
