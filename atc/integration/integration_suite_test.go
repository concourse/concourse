package integration_test

import (
	"net/http"
	"os"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/atc/postgresrunner"
	flags "github.com/jessevdk/go-flags"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"testing"
)

var (
	cmd            *atccmd.RunCommand
	postgresRunner postgresrunner.Runner
	dbProcess      ifrit.Process
)

var _ = BeforeEach(func() {
	cmd = &atccmd.RunCommand{}
	_, err := flags.ParseArgs(cmd, []string{})
	Expect(err).NotTo(HaveOccurred())
	cmd.Postgres.User = "postgres"
	cmd.Postgres.Database = "testdb"
	cmd.Postgres.Port = 5433 + uint16(GinkgoParallelNode())
	cmd.Postgres.SSLMode = "disable"
	cmd.Auth.MainTeamFlags.LocalUsers = []string{"test"}
	cmd.Auth.AuthFlags.LocalUsers = map[string]string{
		"test":   "test",
		"v-user": "v-user",
		"m-user": "m-user",
		"o-user": "o-user",
	}
	cmd.Logger.LogLevel = "debug"
	cmd.Logger.SetWriterSink(GinkgoWriter)
	cmd.BindPort = 9090 + uint16(GinkgoParallelNode())
	cmd.DebugBindPort = 0

	postgresRunner = postgresrunner.Runner{
		Port: 5433 + GinkgoParallelNode(),
	}
	dbProcess = ifrit.Invoke(postgresRunner)
	postgresRunner.CreateTestDB()

	// workaround to avoid panic due to registering http handlers multiple times
	http.DefaultServeMux = new(http.ServeMux)
})

var _ = AfterEach(func() {
	postgresRunner.DropTestDB()

	dbProcess.Signal(os.Interrupt)
	err := <-dbProcess.Wait()
	Expect(err).NotTo(HaveOccurred())
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}
