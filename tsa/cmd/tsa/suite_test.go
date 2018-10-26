package main_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
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
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/tsa"
	jwt "github.com/dgrijalva/jwt-go"
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

type registration struct {
	worker atc.Worker
	ttl    time.Duration
}

var (
	forwardHost string

	tsaPort           int
	tsaDebugPort      int
	heartbeatInterval = 1 * time.Second
	tsaProcess        ifrit.Process

	gardenAddr  string
	fakeBackend *gfakes.FakeBackend

	gardenServer       *gserver.GardenServer
	baggageclaimServer *ghttp.Server
	atcServer          *ghttp.Server

	hostKeyFile    string
	hostPubKey     ssh.PublicKey
	hostPubKeyFile string

	accessFactory      accessor.AccessFactory
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

	registered  chan registration
	heartbeated chan registration
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

	sessionSigningPrivateKeyFile, _, _, _ := generateSSHKeypair()

	rsaKeyBlob, err := ioutil.ReadFile(string(sessionSigningPrivateKeyFile))
	Expect(err).NotTo(HaveOccurred())

	signingKey, err := jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
	Expect(err).NotTo(HaveOccurred())

	accessFactory = accessor.NewAccessFactory(&signingKey.PublicKey)

	tsaCommand := exec.Command(
		tsaPath,
		"--bind-port", strconv.Itoa(tsaPort),
		"--bind-debug-port", strconv.Itoa(tsaDebugPort),
		"--peer-ip", forwardHost,
		"--host-key", hostKeyFile,
		"--authorized-keys", authorizedKeysFile,
		"--team-authorized-keys", "some-team:"+teamPubKeyFile,
		"--team-authorized-keys", "some-other-team:"+otherTeamPubKeyFile,
		"--session-signing-key", sessionSigningPrivateKeyFile,
		"--atc-url", atcServer.URL(),
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

	registered = make(chan registration, 100)
	heartbeated = make(chan registration, 100)

	atcServer.RouteToHandler("POST", "/api/v1/workers", func(w http.ResponseWriter, r *http.Request) {
		var worker atc.Worker
		Expect(accessFactory.Create(r, "some-action").IsAuthenticated()).To(BeTrue())

		err := json.NewDecoder(r.Body).Decode(&worker)
		Expect(err).NotTo(HaveOccurred())

		ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
		Expect(err).NotTo(HaveOccurred())

		registered <- registration{worker, ttl}
	})

	atcServer.RouteToHandler("PUT", "/api/v1/workers/some-worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var worker atc.Worker
		Expect(accessFactory.Create(r, "some-action").IsAuthenticated()).To(BeTrue())

		err := json.NewDecoder(r.Body).Decode(&worker)
		Expect(err).NotTo(HaveOccurred())

		ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
		Expect(err).NotTo(HaveOccurred())

		heartbeated <- registration{worker, ttl}
	})

})

var _ = AfterEach(func() {
	atcServer.Close()
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
