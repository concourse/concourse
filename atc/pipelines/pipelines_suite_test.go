package pipelines_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPipelines(t *testing.T) {
	SetDefaultEventuallyTimeout(10 * time.Second)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipelines Suite")
}
