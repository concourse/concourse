package k8s_test

import (
	"time"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("external workers through separate deployments", func() {

	const publicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC496FSYFcBAKgDtMsBAJiF/6/NxlXKP5UZecyEsedYuTt1GOgJTwaA1qZ1LmHsbfLDE68oDdiM4uvxfI4wtLhz57w3u0jOUxZ2JeF7SVwEf1nVqLn4Gh/f8GUNQGSyIp1zUD5Bx9fq0PAyQ47mt7Ufi84rcf8LKl7nzAIHTcdg2BvTkQN9bUGPaq/Pb1W2bKPAQy4OzXTSIyrAJ89TH2jFeaZfyxQFGbD9jVHH/yl0oiMrDeaRYgccE5II+KY7WoLjsBry/9Qf2ERELKTK4UeIGIqWci9lab1ti+GxFPPiC3krNFjo4jShV4eUs4cNIrjwNrxVaKPXmU6o7Y3Hpayx Concourse"

	var (
		atc Endpoint

		workerKey        string
		tsaPort          string
		webDeployArgs    []string
		workerDeployArgs []string
	)

	JustBeforeEach(func() {
		setReleaseNameAndNamespace("xw")

		By("creating a web only deployment in one namespace")
		tsaPort = "2222"
		helmArgs := append(webDeployArgs,
			"--set=worker.enabled=false",
			"--set=web.tsa.bindPort="+tsaPort,
		)
		deployConcourseChart(releaseName+"-web", helmArgs...)

		By("creating a worker only deployment in another namespace")
		helmArgs = append(workerDeployArgs,
			"--set=postgresql.enabled=false",
			"--set=web.enabled=false",
			"--set=worker.replicas=1",
			"--set=concourse.worker.tsa.hosts[0]="+releaseName+"-web-web-worker-gateway."+releaseName+"-web.svc.cluster.local:"+tsaPort,
		)
		deployConcourseChart(releaseName+"-worker", helmArgs...)

		waitAllPodsInNamespaceToBeReady(namespace + "-worker")
		waitAllPodsInNamespaceToBeReady(namespace + "-web")

		atc = endpointFactory.NewServiceEndpoint(
			namespace+"-web",
			releaseName+"-web-web",
			"8080",
		)

		fly.Login("test", "test", "http://"+atc.Address())

	})

	var waitForRunningWorker = func() {
		By("waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			Should(HaveLen(1))
	}

	var workerDoesntRegister = func() {
		By("worker never registers")
		Consistently(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 1*time.Minute, 10*time.Second).
			Should(HaveLen(0))
	}

	AfterEach(func() {
		atc.Close()
		cleanup(releaseName+"-web", namespace+"-web")
		cleanup(releaseName+"-worker", namespace+"-worker")
	})

	Context("main team worker, webs only allow team workers", func() {
		invalidGenericKey := "ssh-rsa ABCD1234 OnlyTeamWorkers"
		Context("web with correct public key", func() {
			BeforeEach(func() {
				workerKey = publicKey
				webDeployArgs = []string{
					"--set=secrets.teamAuthorizedKeys[0].team=main",
					"--set=secrets.teamAuthorizedKeys[0].key=" + workerKey,
					"--set=secrets.workerKeyPub=" + invalidGenericKey,
				}
				workerDeployArgs = []string{
					"--set=concourse.worker.team=main",
				}
			})

			It("worker registers with team main", func() {
				waitForRunningWorker()
				worker := getRunningWorkers(fly.GetWorkers())
				Expect(worker[0].Team).To(Equal("main"))
			})
		})

		Context("web with invalid public key", func() {
			BeforeEach(func() {
				workerKey = "ssh-rsa 1234ABCD Concourse"
				webDeployArgs = []string{
					"--set=secrets.teamAuthorizedKeys[0].team=main",
					"--set=secrets.teamAuthorizedKeys[0].key=" + workerKey,
					"--set=secrets.workerKeyPub=" + invalidGenericKey,
				}
				workerDeployArgs = []string{
					"--set=concourse.worker.team=main",
				}
			})

			It("worker doesn't register", func() {
				workerDoesntRegister()
			})
		})
	})

	Context("generic worker", func() {
		Context("web with correct public key", func() {
			BeforeEach(func() {
				workerDeployArgs = []string{}
				webDeployArgs = []string{}
			})

			It("worker registers with atc", func() {
				waitForRunningWorker()
				worker := getRunningWorkers(fly.GetWorkers())
				Expect(worker[0].Team).To(Equal(""))
			})
		})

		Context("web with invalid public key", func() {
			BeforeEach(func() {
				workerKey = "ssh-rsa 1234ABCD Concourse"
				webDeployArgs = []string{
					"--set=secrets.workerKeyPub=" + workerKey,
				}
			})

			It("worker doesn't register", func() {
				workerDoesntRegister()
			})
		})
	})
})
