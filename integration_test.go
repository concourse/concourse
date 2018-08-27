package atc_test

import (
	"crypto/rand"
	"crypto/rsa"
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
)

var _ = Describe("ATC Integration Test", func() {
	var (
		dbProcess ifrit.Process
		cmd       *atccmd.RunCommand
	)

	BeforeEach(func() {
		cmd = RunCommand()
	})

	JustBeforeEach(func() {
		postgresRunner := postgresrunner.Runner{Port: 6543}
		dbProcess = ifrit.Invoke(postgresRunner)
		postgresRunner.CreateTestDB()
		go cmd.Execute([]string{})
		Eventually(func() error {
			_, err := http.Get("http://localhost:9090/api/v1/info")
			return err
		}, 20*time.Second).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		dbProcess.Signal(os.Interrupt)
		<-dbProcess.Wait()
	})

	Context("when the bind ip is 0.0.0.0", func() {
		It("successfully redirects logins to localhost", func() {
			jar, err := cookiejar.New(nil)
			Expect(err).NotTo(HaveOccurred())
			client := http.Client{
				Jar: jar,
			}
			resp, err := client.Get("http://127.0.0.1:9090/sky/login")
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
		})
	})
})

func RunCommand() *atccmd.RunCommand {
	cmd := atccmd.RunCommand{}
	_, err := flags.ParseArgs(&cmd, []string{})
	Expect(err).NotTo(HaveOccurred())
	cmd.Postgres.User = "postgres"
	cmd.Postgres.Database = "testdb"
	cmd.Postgres.Port = 6543
	cmd.Postgres.SSLMode = "disable"
	cmd.Auth.MainTeamFlags.AllowAllUsers = true
	cmd.Auth.AuthFlags.LocalUsers = map[string]string{"test": "$2y$10$yh24anANlBzyCu3DFWW1ze5dgbFEf0UE5I/dMxOworxt2QVVmZfty"}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	cmd.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: key}
	cmd.BindPort = 9090
	return &cmd
}
