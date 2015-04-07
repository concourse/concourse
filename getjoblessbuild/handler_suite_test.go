package getjoblessbuild_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGetJoblessBuild(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Get Jobless Build Handler Suite")
}
