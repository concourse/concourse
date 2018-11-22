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

var (
	Environment struct {
		ConcourseImageDigest string `env:"CONCOURSE_IMAGE_DIGEST"`
		ConcourseImageName   string `env:"CONCOURSE_IMAGE_NAME,required"`
		ConcourseImageTag    string `env:"CONCOURSE_IMAGE_TAG"`
		ChartDir             string `env:"CHART_DIR,required"`
	}
	flyPath string
	fly     Fly
)

var _ = SynchronizedBeforeSuite(func() []byte {
	return []byte(BuildBinary())
}, func(data []byte) {
	flyPath = string(data)
})

var _ = BeforeEach(func() {
	err := env.Parse(&Environment)
	Expect(err).ToNot(HaveOccurred())

	By("Checking if kubectl has a context set")
	Wait(Start(nil, "kubectl", "config", "current-context"))

	By("Installing tiller")
	Wait(Start(nil, "helm", "init", "--wait"))
	Wait(Start(nil, "helm", "dependency", "update", Environment.ChartDir))

	tmp, err := ioutil.TempDir("", "topgun-tmp")
	Expect(err).ToNot(HaveOccurred())

	fly = Fly{
		Bin:    flyPath,
		Target: "concourse-topgun-k8s-" + strconv.Itoa(GinkgoParallelNode()),
		Home:   filepath.Join(tmp, "fly-home"),
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

func helmDeploy(releaseName string, args ...string) {
	helmArgs := []string{
		"upgrade",
		"-f",
		path.Join(Environment.ChartDir, "values.yaml"),
		"--install",
		"--force",
		"--set=concourse.web.kubernetes.keepNamespaces=false",
		"--set=image=" + Environment.ConcourseImageName,
		"--set=imageDigest=" + Environment.ConcourseImageDigest,
		"--set=imageTag=" + Environment.ConcourseImageTag}

	helmArgs = append(helmArgs, args...)
	helmArgs = append(helmArgs, releaseName,
		"--wait",
		Environment.ChartDir)

	Wait(Start(nil, "helm", helmArgs...))
}

func helmDestroy(releaseName string) {
	helmArgs := []string{
		"delete",
		releaseName,
	}

	Wait(Start(nil, "helm", helmArgs...))
}

func getPods(releaseName string, flags ...string) []pod {
	var (
		pods podListResponse
		args = append([]string{"get", "pods",
			"--selector=release=" + releaseName,
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

func deletePods(releaseName string, flags ...string) []string {
	var (
		podNames []string
		args     = append([]string{"delete", "pod",
			"--selector=release=" + releaseName,
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

func startPortForwarding(service, port string) (*gexec.Session, string) {
	session := Start(nil, "kubectl", "port-forward", "service/"+service, ":"+port)
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
