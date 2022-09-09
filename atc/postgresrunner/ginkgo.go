package postgresrunner

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

type none struct{}

func GinkgoRunner(runner *Runner) none {
	var dbProcess ifrit.Process

	BeforeSuite(func() {
		InitializeRunnerForGinkgo(runner, &dbProcess)
	})

	AfterSuite(func() {
		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
	})

	return none{}
}

func InitializeRunnerForGinkgo(runner *Runner, dbProcess *ifrit.Process) {
	*runner = Runner{
		Port: 5433 + GinkgoParallelNode(),
	}
	*dbProcess = ifrit.Invoke(*runner)
	runner.InitializeTestDBTemplate()
}

func FinalizeRunnerForGinkgo(runner *Runner, dbProcess *ifrit.Process) {
	(*dbProcess).Signal(os.Interrupt)
	Eventually((*dbProcess).Wait(), 10*time.Second).Should(Receive())
}
