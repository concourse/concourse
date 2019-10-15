package k8s_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/caarlos0/env"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestK8s(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "K8s Suite")
}

type environment struct {
	ChartsDir            string `env:"CHARTS_DIR,required"`
	ConcourseChartDir    string `env:"CONCOURSE_CHART_DIR"`
	ConcourseImageDigest string `env:"CONCOURSE_IMAGE_DIGEST"`
	ConcourseImageName   string `env:"CONCOURSE_IMAGE_NAME,required"`
	ConcourseImageTag    string `env:"CONCOURSE_IMAGE_TAG"`
	FlyPath              string `env:"FLY_PATH"`
	K8sEngine            string `env:"K8S_ENGINE" envDefault:"GKE"`
}

var (
	Environment environment
	fly         FlyCli
	namespace   string
	releaseName string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var parsedEnv environment

	err := env.Parse(&parsedEnv)
	Expect(err).ToNot(HaveOccurred())

	if parsedEnv.FlyPath == "" {
		parsedEnv.FlyPath = BuildBinary()
	}

	if parsedEnv.ConcourseChartDir == "" {
		parsedEnv.ConcourseChartDir = path.Join(parsedEnv.ChartsDir, "stable/concourse")
	}

	By("Checking if kubectl has a context set")
	Wait(Start(nil, "kubectl", "config", "current-context"))

	By("Initializing the client side of helm")
	Wait(Start(nil, "helm", "init", "--client-only"))

	By("Updating the dependencies of the Concourse chart locally")
	Wait(Start(nil, "helm", "dependency", "update", parsedEnv.ConcourseChartDir))

	envBytes, err := json.Marshal(parsedEnv)
	Expect(err).ToNot(HaveOccurred())

	return envBytes
}, func(data []byte) {
	err := json.Unmarshal(data, &Environment)
	Expect(err).ToNot(HaveOccurred())
})

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultConsistentlyDuration(30 * time.Second)

	tmp, err := ioutil.TempDir("", "topgun-tmp")
	Expect(err).ToNot(HaveOccurred())

	fly = FlyCli{
		Bin:    Environment.FlyPath,
		Target: "concourse-topgun-k8s-" + strconv.Itoa(GinkgoParallelNode()),
		Home:   filepath.Join(tmp, "fly-home-"+strconv.Itoa(GinkgoParallelNode())),
	}

	err = os.Mkdir(fly.Home, 0755)
	Expect(err).ToNot(HaveOccurred())
})

func setReleaseNameAndNamespace(description string) {
	rand.Seed(time.Now().UTC().UnixNano())
	releaseName = fmt.Sprintf("topgun-"+description+"-%d", rand.Int63n(100000000))
	namespace = releaseName
}

type pod struct {
	Status struct {
		ContainerStatuses []struct {
			Name  string `json:"name"`
			Ready bool   `json:"ready"`
		} `json:"containerStatuses"`
		Phase  string `json:"phase"`
		HostIp string `json:"hostIP"`
		Ip     string `json:"podIP"`
	} `json:"status"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

type podListResponse struct {
	Items []pod `json:"items"`
}

func helmDeploy(releaseName, namespace, chartDir string, args ...string) *gexec.Session {
	helmArgs := []string{
		"upgrade",
		"--install",
		"--force",
		"--wait",
		"--namespace", namespace,
	}

	helmArgs = append(helmArgs, args...)
	helmArgs = append(helmArgs, releaseName, chartDir)

	sess := Start(nil, "helm", helmArgs...)
	<-sess.Exited
	return sess
}

func helmInstallArgs(args ...string) []string {
	helmArgs := []string{
		"--set=web.livenessProbe.failureThreshold=3",
		"--set=web.livenessProbe.initialDelaySeconds=3",
		"--set=web.livenessProbe.periodSeconds=3",
		"--set=web.livenessProbe.timeoutSeconds=3",
		"--set=concourse.web.kubernetes.keepNamespaces=false",
		"--set=postgresql.persistence.enabled=false",
		"--set=image=" + Environment.ConcourseImageName}

	if Environment.ConcourseImageTag != "" {
		helmArgs = append(helmArgs, "--set=imageTag="+Environment.ConcourseImageTag)
	}

	if Environment.ConcourseImageDigest != "" {
		helmArgs = append(helmArgs, "--set=imageDigest="+Environment.ConcourseImageDigest)
	}

	return append(helmArgs, args...)
}

func deployFailingConcourseChart(releaseName string, expectedErr string, args ...string) {
	helmArgs := helmInstallArgs(args...)
	sess := helmDeploy(releaseName, releaseName, Environment.ConcourseChartDir, helmArgs...)
	Expect(sess.ExitCode()).ToNot(Equal(0))
	Expect(sess.Err).To(gbytes.Say(expectedErr))
}

func deployConcourseChart(releaseName string, args ...string) {
	helmArgs := helmInstallArgs(args...)
	Eventually(func() int {
		sess := helmDeploy(releaseName, releaseName, Environment.ConcourseChartDir, helmArgs...)
		return sess.ExitCode()
	}).Should(BeZero())
}

func helmDestroy(releaseName string) {
	helmArgs := []string{
		"delete",
		"--purge",
		releaseName,
	}

	Wait(Start(nil, "helm", helmArgs...))
}

func getPods(namespace string, flags ...string) []pod {
	var (
		pods podListResponse
		args = append([]string{"get", "pods",
			"--namespace=" + namespace,
			"--output=json",
			"--no-headers"}, flags...)
		session = Start(nil, "kubectl", args...)
	)

	Wait(session)

	err := json.Unmarshal(session.Out.Contents(), &pods)
	Expect(err).ToNot(HaveOccurred())

	return pods.Items
}

func isPodReady(p pod) bool {
	total := len(p.Status.ContainerStatuses)
	actual := 0

	for _, containerStatus := range p.Status.ContainerStatuses {
		if containerStatus.Ready {
			actual++
		}
	}

	return total == actual
}

func waitAllPodsInNamespaceToBeReady(namespace string) {
	Eventually(func() bool {
		expectedPods := getPods(namespace)
		actualPods := getPods(namespace, "--field-selector=status.phase=Running")

		if len(expectedPods) != len(actualPods) {
			return false
		}

		podsReady := 0
		for _, pod := range actualPods {
			if isPodReady(pod) {
				podsReady++
			}
		}

		return podsReady == len(expectedPods)
	}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "expected all pods to be running")
}

func deletePods(namespace string, flags ...string) []string {
	var (
		podNames []string
		args     = append([]string{"delete", "pod",
			"--namespace=" + namespace,
		}, flags...)
		session = Start(nil, "kubectl", args...)
	)

	Wait(session)

	scanner := bufio.NewScanner(bytes.NewBuffer(session.Out.Contents()))
	for scanner.Scan() {
		podNames = append(podNames, scanner.Text())
	}

	return podNames
}

func startPortForwardingWithProtocol(namespace, resource, port, protocol string) (*gexec.Session, string) {
	session := Start(nil, "kubectl", "port-forward", "--namespace="+namespace, resource, ":"+port)
	Eventually(session.Out).Should(gbytes.Say("Forwarding"))

	address := regexp.MustCompile(`127\.0\.0\.1:[0-9]+`).
		FindStringSubmatch(string(session.Out.Contents()))

	Expect(address).NotTo(BeEmpty())

	return session, protocol + "://" + address[0]
}

func startPortForwarding(namespace, resource, port string) (*gexec.Session, string) {
	return startPortForwardingWithProtocol(namespace, resource, port, "http")
}

func getRunningWorkers(workers []Worker) (running []Worker) {
	for _, w := range workers {
		if w.State == "running" {
			running = append(running, w)
		}
	}
	return
}

func cleanup(releaseName, namespace string, proxySession *gexec.Session) {
	helmDestroy(releaseName)
	Run(nil, "kubectl", "delete", "namespace", namespace, "--wait=false")

	if proxySession != nil {
		Wait(proxySession.Interrupt())
	}
}

func onPks(f func()) {
	Context("PKS", func() {

		BeforeEach(func() {
			if Environment.K8sEngine != "PKS" {
				Skip("not running on PKS")
			}
		})

		f()
	})
}

func onGke(f func()) {
	Context("GKE", func() {

		BeforeEach(func() {
			if Environment.K8sEngine != "GKE" {
				Skip("not running on GKE")
			}
		})

		f()
	})
}
