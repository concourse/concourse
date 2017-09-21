package topgun_test

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
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
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"strconv"
	"testing"
)

var (
	deploymentName, flyTarget string
	jobInstances              map[string][]boshInstance

	dbInstance *boshInstance
	dbConn     *sql.DB

	atcInstance    *boshInstance
	atcExternalURL string

	concourseReleaseVersion, gardenRuncReleaseVersion, postgresReleaseVersion string
	gitServerReleaseVersion, vaultReleaseVersion                              string
	stemcellVersion                                                           string

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

	n, found := os.LookupEnv("TOPGUN_NETWORK_OFFSET")
	var networkOffset int
	var err error

	if found {
		networkOffset, err = strconv.Atoi(n)
	}
	Expect(err).NotTo(HaveOccurred())

	concourseReleaseVersion = os.Getenv("CONCOURSE_RELEASE_VERSION")
	if concourseReleaseVersion == "" {
		concourseReleaseVersion = "latest"
	}

	gardenRuncReleaseVersion = os.Getenv("GARDEN_RUNC_RELEASE_VERSION")
	if gardenRuncReleaseVersion == "" {
		gardenRuncReleaseVersion = "latest"
	}

	postgresReleaseVersion = os.Getenv("POSTGRES_RELEASE_VERSION")
	if postgresReleaseVersion == "" {
		postgresReleaseVersion = "latest"
	}

	gitServerReleaseVersion = os.Getenv("GIT_SERVER_RELEASE_VERSION")
	if gitServerReleaseVersion == "" {
		gitServerReleaseVersion = "latest"
	}

	vaultReleaseVersion = os.Getenv("VAULT_RELEASE_VERSION")
	if vaultReleaseVersion == "" {
		vaultReleaseVersion = "latest"
	}

	stemcellVersion = os.Getenv("STEMCELL_VERSION")
	if stemcellVersion == "" {
		stemcellVersion = "latest"
	}

	deploymentNumber := GinkgoParallelNode() + (networkOffset * 4)

	deploymentName = fmt.Sprintf("concourse-topgun-%d", deploymentNumber)
	flyTarget = deploymentName

	bosh("delete-deployment")

	jobInstances = map[string][]boshInstance{}

	dbInstance = nil
	dbConn = nil
	atcInstance = nil
	atcExternalURL = ""
})

var _ = AfterEach(func() {
	if boshLogs != nil {
		boshLogs.Signal(os.Interrupt)
		<-boshLogs.Exited
		boshLogs = nil
	}

	deleteAllContainers()

	bosh("delete-deployment")
})

func StartDeploy(manifest string, args ...string) *gexec.Session {
	return spawnBosh(
		append([]string{
			"deploy", manifest,
			"-v", "deployment_name='" + deploymentName + "'",
			"-v", "concourse_release_version='" + concourseReleaseVersion + "'",
			"-v", "garden_runc_release_version='" + gardenRuncReleaseVersion + "'",
			"-v", "postgres_release_version='" + postgresReleaseVersion + "'",
			"-v", "vault_release_version='" + vaultReleaseVersion + "'",
			"-v", "git_server_release_version='" + gitServerReleaseVersion + "'",
			"-v", "stemcell_version='" + stemcellVersion + "'",
		}, args...)...,
	)
}

func Deploy(manifest string, args ...string) {
	if boshLogs != nil {
		boshLogs.Signal(os.Interrupt)
		<-boshLogs.Exited
		boshLogs = nil
	}

	wait(StartDeploy(manifest, args...))

	jobInstances = loadJobInstances()

	atcInstance = JobInstance("atc")
	if atcInstance != nil {
		atcExternalURL = fmt.Sprintf("http://%s:8080", atcInstance.IP)

		// give some time for atc to bootstrap (Run migrations, etc)
		Eventually(func() int {
			flySession := spawnFly("login", "-c", atcExternalURL)
			<-flySession.Exited
			return flySession.ExitCode()
		}, 2*time.Minute).Should(Equal(0))
	}

	dbInstance = JobInstance("postgresql")

	if dbInstance != nil {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", dbInstance.IP))
		Expect(err).ToNot(HaveOccurred())
	}

	boshLogs = spawnBosh("logs", "-f")
}

func JobInstance(job string) *boshInstance {
	is := jobInstances[job]
	if len(is) == 0 {
		return nil
	}

	return &is[0]
}

func JobInstances(instance string) []boshInstance {
	return jobInstances[instance]
}

type boshInstance struct {
	Name string
	IP   string
}

var instanceRow = regexp.MustCompile(`^([^\s]+)\s+-\s+(\w+)\s+z1\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\s*$`)
var jobRow = regexp.MustCompile(`^([^\s]+)\s+(\w+)\s+(\w+)\s+-\s+-\s*$`)

func loadJobInstances() map[string][]boshInstance {
	session := spawnBosh("instances", "-p")
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	output := string(session.Out.Contents())

	jobInstances := map[string][]boshInstance{}

	lines := strings.Split(output, "\n")
	var instance boshInstance
	for _, line := range lines {
		instanceMatch := instanceRow.FindStringSubmatch(line)
		if len(instanceMatch) > 0 {
			instance = boshInstance{
				Name: instanceMatch[1],
				IP:   instanceMatch[3],
			}

			continue
		}

		jobMatch := jobRow.FindStringSubmatch(line)
		if len(jobMatch) > 0 {
			jobName := jobMatch[2]
			jobInstances[jobName] = append(jobInstances[jobName], instance)
		}
	}

	return jobInstances
}

func bosh(argv ...string) *gexec.Session {
	session := spawnBosh(argv...)
	wait(session)
	return session
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
	return concourse.NewClient(atcExternalURL, httpClient, false)
}

func deleteAllContainers() {
	client := concourseClient()
	workers, err := client.ListWorkers()
	Expect(err).NotTo(HaveOccurred())

	containers, err := client.ListContainers(map[string]string{})
	Expect(err).NotTo(HaveOccurred())

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
}

func flyHijackTask(argv ...string) *gexec.Session {
	cmd := exec.Command(flyBin, append([]string{"-t", flyTarget, "hijack"}, argv...)...)
	hijackIn, err := cmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	hijackS, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() bool {
		taskMatcher := gbytes.Say("type: task")
		matched, err := taskMatcher.Match(hijackS)
		Expect(err).ToNot(HaveOccurred())

		if matched {
			re, err := regexp.Compile("([0-9]): .+ type: task")
			Expect(err).NotTo(HaveOccurred())

			taskNumber := re.FindStringSubmatch(string(hijackS.Out.Contents()))[1]
			fmt.Fprintln(hijackIn, taskNumber)

			return true
		}

		return hijackS.ExitCode() == 0
	}).Should(BeTrue())

	return hijackS
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
	By("running: " + argc + " " + strings.Join(argv, " "))
	cmd := exec.Command(argc, argv...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func spawnInteractive(stdin io.Reader, argc string, argv ...string) *gexec.Session {
	By("interactively running: " + argc + " " + strings.Join(argv, " "))
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

		workers := flyTable("workers")

		for _, worker := range workers {
			name := worker["name"]
			state := worker["state"]

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

func flyTable(argv ...string) []map[string]string {
	session := spawnFly(append([]string{"--print-table-headers"}, argv...)...)
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	result := []map[string]string{}
	var headers []string

	rows := strings.Split(string(session.Out.Contents()), "\n")
	for i, row := range rows {
		if i == 0 {
			headers = splitFlyColumns(row)
			continue
		}
		if row == "" {
			continue
		}

		result = append(result, map[string]string{})
		columns := splitFlyColumns(row)

		Expect(columns).To(HaveLen(len(headers)))

		for j, header := range headers {
			if header == "" || columns[j] == "" {
				continue
			}

			result[i-1][header] = columns[j]
		}
	}

	return result
}

func splitFlyColumns(row string) []string {
	return regexp.MustCompile(`\s{2,}`).Split(strings.TrimSpace(row), -1)
}

func waitForWorkersToBeRunning() {
	Eventually(func() bool {
		workers := flyTable("workers")
		anyNotRunning := false
		for _, worker := range workers {

			state := worker["state"]

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

func containersBy(condition, value string) []string {
	containers := flyTable("containers")

	var handles []string
	for _, c := range containers {
		if c[condition] == value {
			handles = append(handles, c["handle"])
		}
	}

	return handles
}

func workersBy(condition, value string) []string {
	containers := flyTable("workers")

	var handles []string
	for _, c := range containers {
		if c[condition] == value {
			handles = append(handles, c["name"])
		}
	}

	return handles
}

func volumesByResourceType(name string) []string {
	volumes := flyTable("volumes", "-d")

	var handles []string
	for _, v := range volumes {
		if v["type"] == "resource" && strings.HasPrefix(v["identifier"], "name:"+name) {
			handles = append(handles, v["handle"])
		}
	}

	return handles
}

func deleteDeploymentWithForcedDrain() {
	delete := spawnBosh("stop")

	var workers []string
	Eventually(func() []string {
		workers = workersBy("state", "retiring")
		return workers
	}).Should(HaveLen(1))

	fly("prune-worker", "-w", workers[0])

	<-delete.Exited
	Expect(delete.ExitCode()).To(Equal(0))
}
