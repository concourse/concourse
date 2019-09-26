package metric_test

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetric(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metric Suite")
}

var testLogger = lager.NewLogger("test")

var _ = BeforeSuite(func() {
	metric.Initialize(testLogger, "test", map[string]string{}, 1000)
})
