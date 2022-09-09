package integration_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"golang.org/x/oauth2"
)

var (
	cmd            *atccmd.RunCommand
	postgresRunner postgresrunner.Runner
	atcProcess     ifrit.Process
	atcURL         string
)

var _ = postgresrunner.GinkgoRunner(&postgresRunner)

var _ = BeforeEach(func() {
	cmd = &atccmd.RunCommand{}

	// call parseArgs to populate flag defaults but ignore errors so that we can
	// use the required:"true" field annotation
	//
	// use flags.None so that we don't print errors
	parser := flags.NewParser(cmd, flags.None)
	_, _ = parser.ParseArgs([]string{})

	cmd.Postgres.User = "postgres"
	cmd.Postgres.Database = "testdb"
	cmd.Postgres.Port = uint16(postgresRunner.Port)
	cmd.Postgres.SSLMode = "disable"
	cmd.Auth.MainTeamFlags.LocalUsers = []string{"test"}
	cmd.Auth.AuthFlags.LocalUsers = map[string]string{
		"test":    "test",
		"v-user":  "v-user",
		"po-user": "po-user",
		"m-user":  "m-user",
		"o-user":  "o-user",
	}
	cmd.Auth.AuthFlags.Clients = map[string]string{
		"client-id": "client-secret",
	}
	cmd.Server.ClientID = "client-id"
	cmd.Server.ClientSecret = "client-secret"
	cmd.Logger.LogLevel = "debug"
	cmd.Logger.SetWriterSink(GinkgoWriter)
	cmd.BindPort = 9090 + uint16(GinkgoParallelNode())
	cmd.DebugBindPort = 0

	signingKey, err := rsa.GenerateKey(rand.Reader, 1024)
	Expect(err).ToNot(HaveOccurred())

	cmd.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: signingKey}

	postgresRunner.CreateTestDBFromTemplate()

	// workaround to avoid panic due to registering http handlers multiple times
	http.DefaultServeMux = new(http.ServeMux)
})

var _ = JustBeforeEach(func() {
	atcURL = fmt.Sprintf("http://localhost:%v", cmd.BindPort)

	runner, err := cmd.Runner([]string{})
	Expect(err).NotTo(HaveOccurred())

	atcProcess = ifrit.Invoke(runner)

	Eventually(func() error {
		_, err := http.Get(atcURL + "/api/v1/info")
		return err
	}, 20*time.Second).ShouldNot(HaveOccurred())
})

var _ = AfterEach(func() {
	atcProcess.Signal(os.Interrupt)
	err := <-atcProcess.Wait()
	Expect(err).NotTo(HaveOccurred())

	postgresRunner.DropTestDB()
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func login(atcURL, username, password string) concourse.Client {
	oauth2Config := oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint:     oauth2.Endpoint{TokenURL: atcURL + "/sky/issuer/token"},
		Scopes:       []string{"openid", "profile", "federated:id"},
	}

	ctx := context.Background()
	oauthToken, err := oauth2Config.PasswordCredentialsToken(ctx, username, password)
	Expect(err).NotTo(HaveOccurred())

	tokenSource := oauth2.StaticTokenSource(oauthToken)
	httpClient := oauth2.NewClient(ctx, tokenSource)

	return concourse.NewClient(atcURL, httpClient, false)
}

func setupTeam(atcURL string, team atc.Team) {
	ccClient := login(atcURL, "test", "test")
	createdTeam, _, _, _, err := ccClient.Team(team.Name).CreateOrUpdate(team)

	Expect(err).ToNot(HaveOccurred())
	Expect(createdTeam.Name).To(Equal(team.Name))
	Expect(createdTeam.Auth).To(Equal(team.Auth))
}

func setupPipeline(atcURL, teamName string, config []byte) {
	ccClient := login(atcURL, "test", "test")
	_, _, _, err := ccClient.Team(teamName).CreateOrUpdatePipelineConfig(atc.PipelineRef{Name: "pipeline-name"}, "0", config, false)
	Expect(err).ToNot(HaveOccurred())
}
