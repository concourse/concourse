package atc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"

	"github.com/concourse/atc/atccmd"
	"github.com/concourse/atc/postgresrunner"
	"github.com/concourse/flag"
	flags "github.com/jessevdk/go-flags"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"
)

var _ = Describe("ATC Integration Test", func() {
	var (
		postgresRunner postgresrunner.Runner
		dbProcess      ifrit.Process
		atcProcess     ifrit.Process
		cmd            *atccmd.RunCommand
	)

	BeforeEach(func() {
		postgresRunner = postgresrunner.Runner{
			Port: 5433 + GinkgoParallelNode(),
		}
		dbProcess = ifrit.Invoke(postgresRunner)
		postgresRunner.CreateTestDB()

		cmd = RunCommand()
	})

	JustBeforeEach(func() {
		cmd.BindPort = 9090 + uint16(GinkgoParallelNode())
		cmd.DebugBindPort = 8079 + uint16(GinkgoParallelNode())
		runner, _, err := cmd.Runner([]string{})
		Expect(err).NotTo(HaveOccurred())
		atcProcess = ifrit.Invoke(sigmon.New(runner))
		Eventually(func() error {
			_, err := http.Get(fmt.Sprintf("http://localhost:%v/api/v1/info", cmd.BindPort))
			return err
		}, 20*time.Second).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		atcProcess.Signal(os.Interrupt)
		<-atcProcess.Wait()
		dbProcess.Signal(os.Interrupt)
		<-dbProcess.Wait()
	})

	Context("when no signing key is provided", func() {
		It("logs in successfully", func() {
			DoLogin(fmt.Sprintf("http://127.0.0.1:%v/sky/login", cmd.BindPort))
		})
	})

	Context("when the bind ip is 0.0.0.0 and a signing key is provided", func() {
		BeforeEach(func() {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			cmd.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: key}
		})

		It("successfully redirects logins to localhost", func() {
			DoLogin(fmt.Sprintf("http://127.0.0.1:%v/sky/login", cmd.BindPort))
		})
	})
})

func RunCommand() *atccmd.RunCommand {
	cmd := atccmd.RunCommand{}
	_, err := flags.ParseArgs(&cmd, []string{})
	Expect(err).NotTo(HaveOccurred())
	cmd.Postgres.User = "postgres"
	cmd.Postgres.Database = "testdb"
	cmd.Postgres.Port = 5433 + uint16(GinkgoParallelNode())
	cmd.Postgres.SSLMode = "disable"
	cmd.Auth.MainTeamFlags.AllowAllUsers = true
	cmd.Auth.AuthFlags.LocalUsers = map[string]string{"test": "$2y$10$yh24anANlBzyCu3DFWW1ze5dgbFEf0UE5I/dMxOworxt2QVVmZfty"}
	return &cmd
}

func DoLogin(loginURL string) {
	jar, err := cookiejar.New(nil)
	Expect(err).NotTo(HaveOccurred())
	client := http.Client{
		Jar: jar,
	}
	resp, err := client.Get(loginURL)
	Expect(err).NotTo(HaveOccurred())
	location := resp.Request.URL.String()

	data := url.Values{
		"login":    []string{"test"},
		"password": []string{"test"},
	}

	resp, err = client.PostForm(location, data)
	Expect(err).NotTo(HaveOccurred())

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	Expect(string(bodyBytes)).ToNot(ContainSubstring("invalid username and password"))
	Expect(resp.StatusCode).To(Equal(200))
}
