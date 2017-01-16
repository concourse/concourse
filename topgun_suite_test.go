package topgun_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	deploymentName, flyTarget string

	atcIP, atcExternalURL string

	concourseReleaseVersion, gardenRuncReleaseVersion string
	stemcellVersion                                   string

	pipelineName string

	tmpHome string
	flyBin  string

	logger *lagertest.TestLogger

	boshLogs *gexec.Session
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func TestTOPGUN(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TOPGUN Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	flyBinPath, err := gexec.Build("github.com/concourse/fly")
	Expect(err).ToNot(HaveOccurred())

	return []byte(flyBinPath)
}, func(data []byte) {
	flyBin = string(data)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultConsistentlyDuration(time.Minute)
	SetDefaultConsistentlyPollingInterval(time.Second)

	logger = lagertest.NewTestLogger("test")

	concourseReleaseVersion = os.Getenv("CONCOURSE_RELEASE_VERSION")
	if concourseReleaseVersion == "" {
		concourseReleaseVersion = "latest"
	}

	gardenRuncReleaseVersion = os.Getenv("GARDEN_RUNC_RELEASE_VERSION")
	if gardenRuncReleaseVersion == "" {
		gardenRuncReleaseVersion = "latest"
	}

	stemcellVersion = os.Getenv("STEMCELL_VERSION")
	if stemcellVersion == "" {
		stemcellVersion = "latest"
	}

	deploymentName = fmt.Sprintf("concourse-topgun-%d", GinkgoParallelNode())
	flyTarget = deploymentName

	bosh("delete-deployment")

	atcIP = fmt.Sprintf("10.234.%d.2", GinkgoParallelNode())
	atcExternalURL = fmt.Sprintf("http://%s:8080", atcIP)
})

var _ = AfterEach(func() {
	boshLogs.Signal(os.Interrupt)
	<-boshLogs.Exited
	boshLogs = nil

	Expect(deleteAllContainers()).To(Succeed())

	bosh("delete-deployment")
})

func Deploy(manifest string) {
	bosh(
		"deploy", manifest,
		"-v", "deployment-name="+deploymentName,
		"-v", "atc-ip="+atcIP,
		"-v", "atc-external-url="+atcExternalURL,
		"-v", "concourse-release-version="+concourseReleaseVersion,
		"-v", "garden-runc-release-version="+gardenRuncReleaseVersion,
		"-v", "stemcell-version="+stemcellVersion,
	)

	fly("login", "-c", atcExternalURL)

	boshLogs = spawnBosh("logs", "-f")
}

func bosh(argv ...string) {
	wait(spawnBosh(argv...))
}

func spawnBosh(argv ...string) *gexec.Session {
	return spawn("bosh", append([]string{"-n", "-d", deploymentName}, argv...)...)
}

func fly(argv ...string) {
	wait(spawnFly(argv...))
}

func concourseClient() concourse.Client {
	token, err := getATCToken(atcExternalURL)
	Expect(err).NotTo(HaveOccurred())
	httpClient := oauthClient(token)
	return concourse.NewClient(atcExternalURL, httpClient)
}

func deleteAllContainers() error {
	client := concourseClient()
	workers, err := client.ListWorkers()
	if err != nil {
		return err
	}

	containers, err := client.ListContainers(map[string]string{})
	if err != nil {
		return err
	}

	for _, worker := range workers {
		connection := gconn.New("tcp", worker.GardenAddr)
		gardenClient := gclient.New(connection)
		for _, container := range containers {
			if container.WorkerName == worker.Name {
				err = gardenClient.Destroy(container.ID)
				if err != nil {
					logger.Error("failed-to-delete-container", err, lager.Data{"handle": container.ID})
				}
			}
		}
	}
	return nil
}

func spawnFly(argv ...string) *gexec.Session {
	return spawn(flyBin, append([]string{"-t", flyTarget}, argv...)...)
}

func spawnFlyInteractive(stdin io.Reader, argv ...string) *gexec.Session {
	return spawnInteractive(stdin, flyBin, append([]string{"-t", flyTarget}, argv...)...)
}

func run(argc string, argv ...string) {
	wait(spawn(argc, argv...))
}

func spawn(argc string, argv ...string) *gexec.Session {
	cmd := exec.Command(argc, argv...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func spawnInteractive(stdin io.Reader, argc string, argv ...string) *gexec.Session {
	cmd := exec.Command(argc, argv...)
	cmd.Stdin = stdin
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func wait(session *gexec.Session) {
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))
}

func getATCToken(atcURL string) (*atc.AuthToken, error) {
	response, err := http.Get(atcURL + "/api/v1/teams/main/auth/token")
	if err != nil {
		return nil, err
	}

	var token *atc.AuthToken
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func oauthClient(atcToken *atc.AuthToken) *http.Client {
	return &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{
				TokenType:   atcToken.Type,
				AccessToken: atcToken.Value,
			}),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func waitForLandingOrLandedWorker() string {
	return waitForWorkerInState("landing", "landed")
}

func waitForRunningWorker() string {
	return waitForWorkerInState("running")
}

func waitForStalledWorker() string {
	return waitForWorkerInState("stalled")
}

func waitForWorkerInState(desiredStates ...string) string {
	var workerName string
	Eventually(func() string {
		session := spawnFly("workers")
		<-session.Exited

		rows := strings.Split(string(session.Out.Contents()), "\n")
		for _, row := range rows {
			if row == "" {
				continue
			}

			worker := strings.Fields(row)

			name := worker[0]
			state := worker[len(worker)-1]

			anyMatched := false
			for _, desiredState := range desiredStates {
				if state == desiredState {
					anyMatched = true
				}
			}

			if !anyMatched {
				continue
			}

			if workerName != "" {
				Fail("multiple workers in states: " + strings.Join(desiredStates, ", "))
			}

			workerName = name
		}

		return workerName
	}).ShouldNot(BeEmpty())

	return workerName
}

func waitForWorkersToBeRunning() {
	Eventually(func() bool {
		session := spawnFly("workers")
		<-session.Exited

		anyNotRunning := false

		rows := strings.Split(string(session.Out.Contents()), "\n")
		for _, row := range rows {
			if row == "" {
				continue
			}

			worker := strings.Fields(row)

			state := worker[len(worker)-1]

			if state != "running" {
				anyNotRunning = true
			}
		}

		return anyNotRunning
	}).Should(BeFalse())
}

func workersWithContainers() []string {
	client := concourseClient()
	containers, err := client.ListContainers(map[string]string{})
	Expect(err).NotTo(HaveOccurred())

	usedWorkers := map[string]struct{}{}

	for _, container := range containers {
		usedWorkers[container.WorkerName] = struct{}{}
	}

	var workerNames []string
	for worker, _ := range usedWorkers {
		workerNames = append(workerNames, worker)
	}

	return workerNames
}
