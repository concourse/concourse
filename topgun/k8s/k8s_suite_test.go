package k8s_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/caarlos0/env"
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
	fly Fly
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
	Wait(Start("kubectl", "config", "current-context"))

	By("Installing tiller")
	Wait(Start("helm", "init", "--wait"))
	Wait(Start("helm", "dependency", "update", Environment.ChartDir))

	tmp, err := ioutil.TempDir("", "topgun-tmp")
	Expect(err).ToNot(HaveOccurred())

	fly = Fly{
		Bin: flyPath,
		Target: "concourse-topgun-k8s-" + strconv.Itoa(GinkgoParallelNode()),
		Home: filepath.Join(tmp, "fly-home"),
	}

	err = os.Mkdir(fly.Home, 0755)
	Expect(err).ToNot(HaveOccurred())
})
