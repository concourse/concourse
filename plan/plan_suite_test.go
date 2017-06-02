package plan_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPlan(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plan Suite")
}
