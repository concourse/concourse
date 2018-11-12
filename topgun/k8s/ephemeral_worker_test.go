package k8s_test

import (
	"fmt"
	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"path"
	"strconv"
	"time"
)

// TODO
// - add paths to the charts git-resource
// - make use of a separated namespace
// - make use of separate values.yml files

// deploy helm chart						DONE
// - using the digest from dev-image		DONE
// - configure only one worker				DONE
// - configure the worker to be ephemeral	DONE

// wait for a worker to be running
// set the pipeline

// delete the worker's pod
// --- poll or `kubctl get pods --watch` until the pod is terminated
// expect that the worker doesn't have any volumes or containers on it:
// - according to fly volumes ++ fly containers
// - on the actual worker

// delete the helm release


func HelmDeploy(releaseName string){
	helmArgs := []string{
		"upgrade",
		"-f",
		path.Join(Environment.ChartDir,"values.yaml"),
		"--install",
		"--force",
		"--set=concourse.web.kubernetes.keepNamespaces=false",
		"--set=concourse.worker.ephemeral=true",
		"--set=concourse.worker.replicas=1",
		"--set=concourse.worker.baggageclaim.driver=btrfs",
		"--set=image="+Environment.ConcourseImageName,
		"--set=imageDigest="+Environment.ConcourseImageDigest,
		releaseName,
		"--wait",
		Environment.ChartDir,
	}

	Wait(Start("helm", helmArgs...))
}


func HelmDestroy(releaseName string){
	helmArgs := []string{
		"delete",
		releaseName,
	}

	Wait(Start("helm", helmArgs...))
}

func StartKubectlProxy(port int) *gexec.Session {
	session := Start("kubectl", "proxy", "--port", strconv.Itoa(port) )
	Eventually(session.Out).Should(gbytes.Say("Starting to serve on"))
	return session
}

// TODO completely remove this
func StartWebProxy(releaseName string, port int) *gexec.Session{
	session := Start("kubectl", "port-forward", "service/" + releaseName + "-web", fmt.Sprintf("%d:8080", port))
	Eventually(session.Out).Should(gbytes.Say("Forwarding"))
	return session
}

var _ = Describe("Ephemeral workers", func () {
	var (
		proxySession *gexec.Session
		releaseName  string
		atcEndpoint  string
	)

	BeforeEach(func(){
		port := 8080 + GinkgoParallelNode()
		releaseName = fmt.Sprintf("topgun-ephemeral-workers-%d", GinkgoParallelNode())
		HelmDeploy(releaseName)

		// Wait for it to be ready?

		By("Creating the web proxy")
		proxySession = StartWebProxy(releaseName, port)
		atcEndpoint = fmt.Sprintf("http://127.0.0.1:%d", port)

		// proxySession = StartKubectlProxy(port)
		//atcEndpoint = fmt.Sprintf("http://127.0.0.1:%d/api/v1/namespaces/default/services/%s:8080/proxy/",
		//	port, releaseName + "-web")
	})

	AfterEach(func() {
		// HelmDestroy(releaseName)
		Wait(proxySession.Interrupt())
	})

	It("Gets properly cleaned when getting removed and then put back on", func () {
		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

		// prepare fly
		// wait for worker to be there
		By("Waiting for a running worker")
		Eventually(
			getRunningWorkers(fly.GetWorkers()), 2 * time.Minute, 10 * time.Second).
			ShouldNot(HaveLen(0))

		By("setting pipeline that creates resource config")
		fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "resource-gc-test")

		By("unpausing the pipeline")
		fly.Run("unpause-pipeline", "-p", "resource-gc-test")

		By("checking resource")
		fly.Run("check-resource", "-r", "resource-gc-test/tick-tock")

		// kubectl
		// delete the worker's pod
	})
})

func getRunningWorkers(workers []Worker) (running []Worker) {
	for _, w := range workers{
		if w.State == "running"{
			running = append(running, w)
		}
	}
    return
}