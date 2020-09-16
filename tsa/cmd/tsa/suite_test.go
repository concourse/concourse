package main_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	gserver "code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/localip"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/tsa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"golang.org/x/crypto/ssh"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var tsaPath string

var _ = BeforeSuite(func() {
	var err error
	tsaPath, err = gexec.Build("github.com/concourse/concourse/tsa/cmd/tsa")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var (
	forwardHost string

	tsaPort           int
	tsaDebugPort      int
	heartbeatInterval = 1 * time.Second
	tsaProcess        ifrit.Process

	gardenRequestTimeout = 3 * time.Second

	gardenAddr  string
	fakeBackend *gfakes.FakeBackend

	gardenServer       *gserver.GardenServer
	baggageclaimServer *ghttp.Server
	atcServer          *ghttp.Server
	authServer         *ghttp.Server

	hostKeyFile    string
	hostPubKey     ssh.PublicKey
	hostPubKeyFile string

	authorizedKeysFile string

	globalKey           *rsa.PrivateKey
	globalKeyFile       string
	teamKey             *rsa.PrivateKey
	teamKeyFile         string
	teamPubKeyFile      string
	otherTeamKey        *rsa.PrivateKey
	otherTeamKeyFile    string
	otherTeamPubKeyFile string

	tsaRunner *ginkgomon.Runner
	tsaClient *tsa.Client
)

var _ = BeforeEach(func() {
	tsaPort = 9800 + GinkgoParallelNode()
	tsaDebugPort = 9900 + GinkgoParallelNode()

	gardenPort := 9001 + GinkgoParallelNode()
	gardenAddr = fmt.Sprintf("127.0.0.1:%d", gardenPort)

	fakeBackend = new(gfakes.FakeBackend)

	gardenServer = gserver.New("tcp", gardenAddr, 0, fakeBackend, lagertest.NewTestLogger("garden"))
	go func() {
		defer GinkgoRecover()
		err := gardenServer.ListenAndServe()
		Expect(err).NotTo(HaveOccurred())
	}()

	apiClient := gclient.New(gconn.New("tcp", gardenAddr))
	Eventually(apiClient.Ping).Should(Succeed())

	err := gardenServer.SetupBomberman()
	Expect(err).NotTo(HaveOccurred())

	baggageclaimServer = ghttp.NewServer()

	atcServer = ghttp.NewServer()
	authServer = ghttp.NewServer()

	authServer.AppendHandlers(ghttp.CombineHandlers(
		ghttp.VerifyRequest("POST", "/token"),
		ghttp.VerifyBasicAuth("some-client", "some-client-secret"),
		ghttp.RespondWithJSONEncoded(200, map[string]string{
			"token_type":   "bearer",
			"access_token": "access-token",
			"id_token":     "id-token",
		}),
	))

	hostKeyFile, hostPubKeyFile, _, hostPubKey = generateSSHKeypair()

	globalKeyFile, _, globalKey, _ = generateSSHKeypair()

	teamKeyFile, teamPubKeyFile, teamKey, _ = generateSSHKeypair()
	otherTeamKeyFile, otherTeamPubKeyFile, otherTeamKey, _ = generateSSHKeypair()

	authorizedKeys, err := ioutil.TempFile("", "authorized-keys")
	Expect(err).NotTo(HaveOccurred())

	defer authorizedKeys.Close()

	authorizedKeysFile = authorizedKeys.Name()

	userPrivateKeyBytes, err := ioutil.ReadFile(globalKeyFile)
	Expect(err).NotTo(HaveOccurred())

	userSigner, err := ssh.ParsePrivateKey(userPrivateKeyBytes)
	Expect(err).NotTo(HaveOccurred())

	_, err = authorizedKeys.Write(ssh.MarshalAuthorizedKey(userSigner.PublicKey()))
	Expect(err).NotTo(HaveOccurred())

	forwardHost, err = localip.LocalIP()
	Expect(err).NotTo(HaveOccurred())

	tsaCommand := exec.Command(
		tsaPath,
		"--bind-port", strconv.Itoa(tsaPort),
		"--peer-address", forwardHost,
		"--debug-bind-port", strconv.Itoa(tsaDebugPort),
		"--host-key", hostKeyFile,
		"--authorized-keys", authorizedKeysFile,
		"--team-authorized-keys", "some-team:"+teamPubKeyFile,
		"--team-authorized-keys", "some-other-team:"+otherTeamPubKeyFile,
		"--client-id", "some-client",
		"--client-secret", "some-client-secret",
		"--token-url", authServer.URL()+"/token",
		"--atc-url", atcServer.URL(),
		"--garden-request-timeout", gardenRequestTimeout.String(),
		"--heartbeat-interval", heartbeatInterval.String(),
	)

	tsaRunner = ginkgomon.New(ginkgomon.Config{
		Command:       tsaCommand,
		Name:          "tsa",
		StartCheck:    "tsa.listening",
		AnsiColorCode: "32m",
	})

	tsaClient = &tsa.Client{
		Hosts:    []string{fmt.Sprintf("127.0.0.1:%d", tsaPort)},
		HostKeys: []ssh.PublicKey{hostPubKey},

		Worker: atc.Worker{
			Name: "some-worker",

			Platform: "linux",
			Tags:     []string{"some", "tags"},

			ResourceTypes: []atc.WorkerResourceType{
				{Type: "resource-type-a", Image: "resource-image-a"},
				{Type: "resource-type-b", Image: "resource-image-b"},
			},
		},
	}

	tsaProcess = ginkgomon.Invoke(tsaRunner)
})

var _ = AfterEach(func() {
	atcServer.Close()
	authServer.Close()
	gardenServer.Stop()
	baggageclaimServer.Close()
	ginkgomon.Interrupt(tsaProcess)
})

func generateSSHKeypair() (string, string, *rsa.PrivateKey, ssh.PublicKey) {
	path, err := ioutil.TempDir("", "tsa-key")
	Expect(err).NotTo(HaveOccurred())

	privateKeyPath := filepath.Join(path, "id_rsa")
	publicKeyPath := privateKeyPath + ".pub"

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	privateKeyBytes := pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKeyRsa, err := ssh.NewPublicKey(&privateKey.PublicKey)
	Expect(err).NotTo(HaveOccurred())

	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKeyRsa)

	err = ioutil.WriteFile(privateKeyPath, privateKeyBytes, 0600)
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(publicKeyPath, publicKeyBytes, 0600)
	Expect(err).NotTo(HaveOccurred())

	return privateKeyPath, publicKeyPath, privateKey, publicKeyRsa
}
