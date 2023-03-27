package metric_test

import (
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetric(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metric Suite")
}

var testLogger = lager.NewLogger("test")
