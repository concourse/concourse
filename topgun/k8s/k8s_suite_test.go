package k8s_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/caarlos0/env"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

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
}

var (
	Environment environment
	fly         Fly
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

	By("Installing tiller in the k8s cluster")
	Wait(Start(nil, "helm", "init", "--wait"))

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
	tmp, err := ioutil.TempDir("", "topgun-tmp")
	Expect(err).ToNot(HaveOccurred())

	fly = Fly{
		Bin:    Environment.FlyPath,
		Target: "concourse-topgun-k8s-" + strconv.Itoa(GinkgoParallelNode()),
		Home:   filepath.Join(tmp, "fly-home-"+strconv.Itoa(GinkgoParallelNode())),
	}

	err = os.Mkdir(fly.Home, 0755)
	Expect(err).ToNot(HaveOccurred())
})

type pod struct {
	Status struct {
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

func helmDeploy(releaseName, chartDir string, args ...string) {
	helmArgs := []string{
		"upgrade",
		"--install",
		"--force",
		"--wait",
		"--namespace", releaseName,
	}

	helmArgs = append(helmArgs, args...)
	helmArgs = append(helmArgs, releaseName, chartDir)

	Wait(Start(nil, "helm", helmArgs...))
}

func deployConcourseChart(releaseName string, args ...string) {
	helmArgs := []string{
		"--set=postgresql.persistence.enabled=false",
		"--set=concourse.web.kubernetes.keepNamespaces=false",
		"--set=concourse.web.livenessProbe.initialDelaySeconds=1s",
		"--set=concourse.web.livenessProbe.periodSeconds=3s",
		"--set=concourse.web.livenessProbe.failureThreshold=30",
		"--set=concourse.web.readinessProbe.initialDelaySeconds=1s",
		"--set=concourse.web.readinessProbe.periodSeconds=3s",
		"--set=concourse.web.readinessProbe.failureThreshold=30",
		"--set=image=" + Environment.ConcourseImageName,
		"--set=imageTag=" + Environment.ConcourseImageTag}

	if Environment.ConcourseImageDigest != "" {
		helmArgs = append(helmArgs, "--set=imageDigest="+Environment.ConcourseImageDigest)
	}

	helmArgs = append(helmArgs, args...)
	helmDeploy(releaseName, Environment.ConcourseChartDir, helmArgs...)
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

func getPodsNames(pods []pod) []string {
	var names []string

	for _, pod := range pods {
		names = append(names, pod.Metadata.Name)
	}

	return names
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

func startPortForwarding(namespace, service, port string) (*gexec.Session, string) {
	session := Start(nil, "kubectl", "port-forward", "--namespace="+namespace, "service/"+service, ":"+port)
	Eventually(session.Out).Should(gbytes.Say("Forwarding"))

	address := regexp.MustCompile(`127\.0\.0\.1:[0-9]+`).
		FindStringSubmatch(string(session.Out.Contents()))

	Expect(address).NotTo(BeEmpty())

	return session, "http://" + address[0]
}

func getRunningWorkers(workers []Worker) (running []Worker) {
	for _, w := range workers {
		if w.State == "running" {
			running = append(running, w)
		}
	}
	return
}
