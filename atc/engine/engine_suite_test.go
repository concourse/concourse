package engine_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func init() {
	util.PanicSink = GinkgoWriter
}

func TestEngine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Engine Suite")
}

var noopStepper exec.Stepper = func(atc.Plan) exec.Step {
	Fail("cannot create substep")
	return nil
}
