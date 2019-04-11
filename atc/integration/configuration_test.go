package integration_test

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

	"github.com/concourse/flag"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("ATC Integration Test", func() {
	var (
		atcProcess ifrit.Process
		atcURL     string
	)

	JustBeforeEach(func() {
		atcURL = fmt.Sprintf("http://127.0.0.1:%v", cmd.BindPort)

		runner, err := cmd.Runner([]string{})
		Expect(err).NotTo(HaveOccurred())

		atcProcess = ginkgomon.Invoke(runner)

		Eventually(func() error {
			_, err := http.Get(atcURL + "/api/v1/info")
			return err
		}, 20*time.Second).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		atcProcess.Signal(os.Interrupt)
		<-atcProcess.Wait()
	})

	Context("when no signing key is provided", func() {
		It("logs in successfully", func() {
			webLogin(atcURL, "test", "test")
		})
	})

	Context("when the bind ip is 0.0.0.0 and a signing key is provided", func() {
		BeforeEach(func() {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			cmd.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: key}
		})

		It("successfully redirects logins to localhost", func() {
			webLogin(atcURL, "test", "test")
		})
	})

	Context("when instance name is specified", func() {
		BeforeEach(func() {
			cmd.Server.InstanceName = "foobar"
		})

		It("renders instance name into HTML template", func() {
			resp, err := http.Get(atcURL)
			Expect(err).NotTo(HaveOccurred())

			bodyBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))
			Expect(string(bodyBytes)).To(ContainSubstring("foobar"))
		})
	})

	It("set default team and config auth for the main team", func() {
		client := webLogin(atcURL, "test", "test")

		resp, err := client.Get(atcURL + "/api/v1/teams")
		Expect(err).NotTo(HaveOccurred())

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))
		Expect(string(bodyBytes)).To(ContainSubstring("main"))
		Expect(string(bodyBytes)).To(ContainSubstring("local:test"))
	})
})

func webLogin(atcURL, username, password string) http.Client {

	jar, err := cookiejar.New(nil)
	Expect(err).NotTo(HaveOccurred())
	client := http.Client{
		Jar: jar,
	}
	resp, err := client.Get(atcURL + "/sky/login")

	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))
	location := resp.Request.URL.String()

	data := url.Values{
		"login":    []string{username},
		"password": []string{password},
	}

	resp, err = client.PostForm(location, data)
	Expect(err).NotTo(HaveOccurred())

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	Expect(resp.StatusCode).To(Equal(200))
	Expect(string(bodyBytes)).ToNot(ContainSubstring("invalid username and password"))

	return client
}
