package integration_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/square/certstrap/pkix"
)

var (
	flyPath string
	homeDir string

	atcServer *ghttp.Server

	serverCert, serverKey []byte
)

const targetName = "testserver"
const teamName = "main"
const atcVersion = "6.3.1"
const workerVersion = "4.5.6"
const bytesSeparator = "BYTES_SEPARATOR"

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("github.com/concourse/concourse/fly", "-buildvcs=false")
	Expect(err).NotTo(HaveOccurred())

	key, err := pkix.CreateRSAKey(1024)
	Expect(err).ToNot(HaveOccurred())

	ca, err := pkix.CreateCertificateAuthority(key, "", time.Now().Add(time.Hour), "", "", "", "", "server-ca", nil)
	Expect(err).ToNot(HaveOccurred())

	serversKey, err := pkix.CreateRSAKey(1024)
	Expect(err).ToNot(HaveOccurred())

	serverKeyBytes, err := serversKey.ExportPrivate()
	Expect(err).ToNot(HaveOccurred())

	serverName := "server"

	serverCSR, err := pkix.CreateCertificateSigningRequest(serversKey, "", []net.IP{net.ParseIP("127.0.0.1")}, []string{serverName}, nil, "", "", "", "", "")
	Expect(err).ToNot(HaveOccurred())

	serversCert, err := pkix.CreateCertificateHost(ca, key, serverCSR, time.Now().Add(time.Hour))
	Expect(err).ToNot(HaveOccurred())
	serverCertBytes, err := serversCert.Export()
	Expect(err).ToNot(HaveOccurred())

	return bytes.Join([][]byte{
		[]byte(binPath),
		serverCertBytes,
		serverKeyBytes,
	}, []byte(bytesSeparator))
},
	func(data []byte) {
		splitData := bytes.Split(data, []byte(bytesSeparator))
		Expect(splitData).To(HaveLen(3))

		flyPath = string(splitData[0])
		serverCert = splitData[1]
		serverKey = splitData[2]

		os.Setenv("FLY_TEST", "true")
	})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

func infoHandler() http.HandlerFunc {
	GinkgoHelper()
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", "/api/v1/info"),
		ghttp.RespondWithJSONEncoded(200, atc.Info{Version: atcVersion, WorkerVersion: workerVersion}),
	)
}

func tokenHandler() http.HandlerFunc {
	GinkgoHelper()
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("POST", "/sky/issuer/token"),
		ghttp.RespondWithJSONEncoded(
			200,
			oauthToken(),
		),
	)
}

func userInfoHandler() http.HandlerFunc {
	GinkgoHelper()
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET", "/api/v1/user"),
		ghttp.RespondWithJSONEncoded(200, map[string]any{
			"user_name": "user",
			"teams": map[string][]string{
				teamName:          {"owner"},
				"some-team":       {"owner"},
				"some-other-team": {"owner"},
			},
		}),
	)
}

func validAccessToken(expiry time.Time) string {
	GinkgoHelper()
	accessToken, err := token.Factory{}.GenerateAccessToken(db.Claims{
		Claims: jwt.Claims{Expiry: jwt.NewNumericDate(expiry)}},
	)
	if err != nil {
		panic(err)
	}
	return accessToken
}

func oauthToken() map[string]string {
	GinkgoHelper()
	return map[string]string{
		"token_type":   "Bearer",
		"access_token": validAccessToken(time.Now()),
		"id_token":     "some-token",
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func createFlyRc(targets rc.Targets) {
	flyrc := filepath.Join(homeDir, ".flyrc")

	flyrcBytes, err := json.Marshal(rc.RC{Targets: targets})
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(flyrc, flyrcBytes, 0600)
	if err != nil {
		panic(err)
	}
}

var _ = BeforeEach(func() {
	atcServer = ghttp.NewServer()

	atcServer.AppendHandlers(
		infoHandler(),
		tokenHandler(),
		userInfoHandler(),
		infoHandler(),
	)

	var err error

	homeDir, err = os.MkdirTemp("", "fly-test")
	Expect(err).NotTo(HaveOccurred())

	os.Setenv("HOME", homeDir)
	loginCmd := exec.Command(flyPath, "-t", targetName, "login", "-u", "user", "-p", "pass", "-c", atcServer.URL(), "-n", teamName)

	session, err := gexec.Start(loginCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session).Should(gexec.Exit(0))

})

var _ = AfterEach(func() {
	atcServer.Close()
	os.RemoveAll(homeDir)
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(10 * time.Second)
	RunSpecs(t, "Fly Integration Suite")
}

func osFlag(short string, long string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("/%s, /%s", short, long)
	} else {
		return fmt.Sprintf("-%s, --%s", short, long)
	}
}

func Change(fn func() int) *changeMatcher {
	return &changeMatcher{
		fn: fn,
	}
}

type changeMatcher struct {
	fn     func() int
	amount int

	before int
	after  int
}

func (cm *changeMatcher) By(amount int) *changeMatcher {
	cm.amount = amount

	return cm
}

func (cm *changeMatcher) Match(actual any) (success bool, err error) {
	cm.before = cm.fn()

	ac, ok := actual.(func())
	if !ok {
		return false, errors.New("expected a function")
	}

	ac()

	cm.after = cm.fn()

	return (cm.after - cm.before) == cm.amount, nil
}

func (cm *changeMatcher) FailureMessage(actual any) (message string) {
	return fmt.Sprintf("Expected value to change by %d but it changed from %d to %d", cm.amount, cm.before, cm.after)
}

func (cm *changeMatcher) NegatedFailureMessage(actual any) (message string) {
	return fmt.Sprintf("Expected value not to change by %d but it changed from %d to %d", cm.amount, cm.before, cm.after)
}
