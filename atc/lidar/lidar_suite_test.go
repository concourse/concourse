package lidar_test

import (
	"github.com/concourse/concourse/atc/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func init() {
	util.PanicSink = GinkgoWriter
}

func TestLidar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lidar Suite")
}
