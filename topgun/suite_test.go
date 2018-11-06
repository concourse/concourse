package topgun_test

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/onsi/gomega/gexec"
	"golang.org/x/oauth2"
)

var (
	deploymentName, flyTarget string
	instances                 map[string][]boshInstance
	jobInstances              map[string][]boshInstance

	dbInstance *boshInstance
	dbConn     *sql.DB

	webInstance    *boshInstance
	atcExternalURL string
	atcUsername    string
	atcPassword    string

	workerGardenClient       gclient.Client
	workerBaggageclaimClient bclient.Client

	concourseReleaseVersion, bpmReleaseVersion, postgresReleaseVersion  string
	gitServerReleaseVersion, vaultReleaseVersion, credhubReleaseVersion string
	stemcellVersion                                                     string

	pipelineName string

	flyBin  string
	flyHome string

	logger *lagertest.TestLogger

	tmp string
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func TestTOPGUN(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TOPGUN Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	flyBinPath, err := gexec.Build("github.com/concourse/concourse/fly")
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
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultConsistentlyDuration(time.Minute)
	SetDefaultConsistentlyPollingInterval(time.Second)

	logger = lagertest.NewTestLogger("test")

	concourseReleaseVersion = os.Getenv("CONCOURSE_RELEASE_VERSION")
	if concourseReleaseVersion == "" {
		concourseReleaseVersion = "latest"
	}

	bpmReleaseVersion = os.Getenv("BPM_RELEASE_VERSION")
	if bpmReleaseVersion == "" {
		bpmReleaseVersion = "latest"
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

	credhubReleaseVersion = os.Getenv("CREDHUB_RELEASE_VERSION")
	if credhubReleaseVersion == "" {
		credhubReleaseVersion = "latest"
	}

	stemcellVersion = os.Getenv("STEMCELL_VERSION")
	if stemcellVersion == "" {
		stemcellVersion = "latest"
	}

	deploymentNumber := GinkgoParallelNode()

	deploymentName = fmt.Sprintf("concourse-topgun-%d", deploymentNumber)
	flyTarget = deploymentName

	var err error
	tmp, err = ioutil.TempDir("", "topgun-tmp")
	Expect(err).ToNot(HaveOccurred())

	flyHome = filepath.Join(tmp, "fly-home")
	err = os.Mkdir(flyHome, 0755)
	Expect(err).ToNot(HaveOccurred())

	waitForDeploymentLock()
	bosh("delete-deployment", "--force")

	instances = map[string][]boshInstance{}
	jobInstances = map[string][]boshInstance{}

	dbInstance = nil
	dbConn = nil
	webInstance = nil
	atcExternalURL = ""
	atcUsername = "test"
	atcPassword = "test"
})

var _ = AfterEach(func() {
	test := CurrentGinkgoTestDescription()
	if test.Failed {
		dir := filepath.Join("logs", fmt.Sprintf("%s.%d", filepath.Base(test.FileName), test.LineNumber))

		err := os.MkdirAll(dir, 0755)
		Expect(err).ToNot(HaveOccurred())

		TimestampedBy("saving logs to " + dir + " due to test failure")
		bosh("logs", "--dir", dir)
	}

	deleteAllContainers()

	waitForDeploymentLock()
	bosh("delete-deployment")

	Expect(os.RemoveAll(tmp)).To(Succeed())
})

func requestCredsInfo(webUrl string) ([]byte, error) {
	request, err := http.NewRequest("GET", webUrl+"/api/v1/info/creds", nil)
	Expect(err).ToNot(HaveOccurred())

	reqHeader := http.Header{}
	token, err := fetchToken(webUrl, atcUsername, atcPassword)
	Expect(err).ToNot(HaveOccurred())

	reqHeader.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header = reqHeader

	client := &http.Client{}
	resp, err := client.Do(request)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))

	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return body, err
}

func StartDeploy(manifest string, args ...string) *gexec.Session {
	waitForDeploymentLock()

	return spawnBosh(
		append([]string{
			"deploy", manifest,
			"--vars-store", filepath.Join(tmp, deploymentName+"-vars.yml"),
			"-v", "deployment_name='" + deploymentName + "'",
			"-v", "concourse_release_version='" + concourseReleaseVersion + "'",
			"-v", "bpm_release_version='" + bpmReleaseVersion + "'",
			"-v", "postgres_release_version='" + postgresReleaseVersion + "'",
			"-v", "vault_release_version='" + vaultReleaseVersion + "'",
			"-v", "credhub_release_version='" + credhubReleaseVersion + "'",
			"-v", "git_server_release_version='" + gitServerReleaseVersion + "'",
			"-v", "stemcell_version='" + stemcellVersion + "'",
		}, args...)...,
	)
}

func Deploy(manifest string, args ...string) {
	if dbConn != nil {
		Expect(dbConn.Close()).To(Succeed())
	}

	wait(StartDeploy(manifest, args...))

	instances, jobInstances = loadJobInstances()

	webInstance = JobInstance("web")
	if webInstance != nil {
		atcExternalURL = fmt.Sprintf("http://%s:8080", webInstance.IP)
		FlyLogin(atcExternalURL)

		waitForWorkersToBeRunning(len(JobInstances("worker")) + len(JobInstances("other_worker")))

		workers := flyTable("workers", "-d")
		if len(workers) > 0 {
			worker := workers[0]
			workerGardenClient = gclient.New(gconn.New("tcp", worker["garden address"]))
			workerBaggageclaimClient = bclient.NewWithHTTPClient(worker["baggageclaim url"], http.DefaultClient)
		} else {
			workerGardenClient = nil
			workerBaggageclaimClient = nil
		}
	}

	dbInstance = JobInstance("postgres")

	if dbInstance != nil {
		var err error
		dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", dbInstance.IP))
		Expect(err).ToNot(HaveOccurred())
	}
}

func Instance(name string) *boshInstance {
	is := instances[name]
	if len(is) == 0 {
		return nil
	}

	return &is[0]
}

func Instances(name string) []boshInstance {
	return instances[name]
}

func JobInstance(job string) *boshInstance {
	is := jobInstances[job]
	if len(is) == 0 {
		return nil
	}

	return &is[0]
}

func JobInstances(job string) []boshInstance {
	return jobInstances[job]
}

type boshInstance struct {
	Name  string
	Group string
	ID    string
	IP    string
}

var instanceRow = regexp.MustCompile(`^([^/]+)/([^\s]+)\s+-\s+(\w+)\s+z1\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\s*$`)
var jobRow = regexp.MustCompile(`^([^\s]+)\s+(\w+)\s+(\w+)\s+-\s+-\s*$`)

func loadJobInstances() (map[string][]boshInstance, map[string][]boshInstance) {
	session := spawnBosh("instances", "-p")
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	output := string(session.Out.Contents())

	instances := map[string][]boshInstance{}
	jobInstances := map[string][]boshInstance{}

	lines := strings.Split(output, "\n")
	var instance boshInstance
	for _, line := range lines {
		instanceMatch := instanceRow.FindStringSubmatch(line)
		if len(instanceMatch) > 0 {
			group := instanceMatch[1]
			id := instanceMatch[2]

			instance = boshInstance{
				Name:  group + "/" + id,
				Group: group,
				ID:    id,
				IP:    instanceMatch[4],
			}

			instances[group] = append(instances[group], instance)

			continue
		}

		jobMatch := jobRow.FindStringSubmatch(line)
		if len(jobMatch) > 0 {
			jobName := jobMatch[2]
			jobInstances[jobName] = append(jobInstances[jobName], instance)
		}
	}

	return instances, jobInstances
}

func bosh(argv ...string) *gexec.Session {
	session := spawnBosh(argv...)
	wait(session)
	return session
}

func spawnBosh(argv ...string) *gexec.Session {
	return spawn("bosh", nil, append([]string{"-n", "-d", deploymentName}, argv...)...)
}

func fly(argv ...string) *gexec.Session {
	session := spawnFly(argv...)
	wait(session)
	return session
}

func concourseClient() concourse.Client {
	token, err := fetchToken(atcExternalURL, atcUsername, atcPassword)
	Expect(err).NotTo(HaveOccurred())

	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	return concourse.NewClient(atcExternalURL, httpClient, false)
}

func fetchToken(webURL string, username, password string) (*oauth2.Token, error) {
	oauth2Config := oauth2.Config{
		ClientID:     "fly",
		ClientSecret: "Zmx5",
		Endpoint:     oauth2.Endpoint{TokenURL: webURL + "/sky/token"},
		Scopes:       []string{"openid", "profile", "email", "federated:id"},
	}

	return oauth2Config.PasswordCredentialsToken(context.Background(), username, password)
}

func deleteAllContainers() {
	client := concourseClient()
	workers, err := client.ListWorkers()
	Expect(err).NotTo(HaveOccurred())

	mainTeam := client.Team("main")
	containers, err := mainTeam.ListContainers(map[string]string{})
	Expect(err).NotTo(HaveOccurred())

	for _, worker := range workers {
		if worker.GardenAddr == "" {
			continue
		}

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

func FlyLogin(endpoint string) {
	Eventually(func() *gexec.Session {
		return spawnFly(
			"login",
			"-c", endpoint,
			"-u", atcUsername,
			"-p", atcPassword,
		).Wait()
	}, 2*time.Minute).Should(gexec.Exit(0), "fly should have been able to log in")
}

func spawnFly(argv ...string) *gexec.Session {
	return spawn(flyBin, []string{"HOME=" + flyHome}, append([]string{"-t", flyTarget}, argv...)...)
}

func spawnFlyInteractive(stdin io.Reader, argv ...string) *gexec.Session {
	return spawnInteractive(stdin, []string{"HOME=" + flyHome}, flyBin, append([]string{"-t", flyTarget}, argv...)...)
}

func TimestampedBy(msg string) {
	By(fmt.Sprintf("[%.9f] %s", float64(time.Now().UnixNano())/1e9, msg))
}

func spawn(argc string, env []string, argv ...string) *gexec.Session {
	cmd := exec.Command(argc, argv...)
	cmd.Env = env

	TimestampedBy("running: " + argc + " " + strings.Join(argv, " "))
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func spawnInteractive(stdin io.Reader, env []string, argc string, argv ...string) *gexec.Session {
	cmd := exec.Command(argc, argv...)
	cmd.Stdin = stdin
	cmd.Env = env

	TimestampedBy("interactively running: " + argc + " " + strings.Join(argv, " "))
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func wait(session *gexec.Session) {
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))
}

func waitForLandedWorker() string {
	return waitForWorkerInState("landed")
}

func waitForRunningWorker() string {
	return waitForWorkerInState("running")
}

func waitForStalledWorker() string {
	return waitForWorkerInState("stalled")
}

func workerState(name string) string {
	workers := flyTable("workers")

	for _, w := range workers {
		if w["name"] == name {
			return w["state"]
		}
	}

	return ""
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
	}, 2*time.Minute, 5*time.Second).ShouldNot(BeEmpty(), "should have seen a worker in states: "+strings.Join(desiredStates, ", "))

	return workerName
}

func flyTable(argv ...string) []map[string]string {
	session := spawnFly(append([]string{"--print-table-headers"}, argv...)...)
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	result := []map[string]string{}

	var headers []string
	for i, cols := range parseTable(string(session.Out.Contents())) {
		if i == 0 {
			headers = cols
			continue
		}

		result = append(result, map[string]string{})

		for j, header := range headers {
			if header == "" || cols[j] == "" {
				continue
			}

			result[i-1][header] = cols[j]
		}
	}

	return result
}

func parseTable(content string) [][]string {
	result := [][]string{}

	var expectedColumns int
	rows := strings.Split(content, "\n")
	for i, row := range rows {
		if row == "" {
			continue
		}

		columns := splitTableColumns(row)
		if i == 0 {
			expectedColumns = len(columns)
		} else {
			Expect(columns).To(HaveLen(expectedColumns))
		}

		result = append(result, columns)
	}

	return result
}

func splitTableColumns(row string) []string {
	return regexp.MustCompile(`(\s{2,}|\t)`).Split(strings.TrimSpace(row), -1)
}

func waitForWorkersToBeRunning(expected int) {
	Eventually(func() interface{} {
		workers := flyTable("workers")

		runningWorkers := []map[string]string{}
		for _, worker := range workers {
			if worker["state"] == "running" {
				runningWorkers = append(runningWorkers, worker)
			}
		}

		return runningWorkers
	}).Should(HaveLen(expected), "expected all workers to be running")
}

func workersWithContainers() []string {
	mainTeam := concourseClient().Team("main")
	containers, err := mainTeam.ListContainers(map[string]string{})
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

func waitForDeploymentLock() {
dance:
	for {
		locks := bosh("locks", "--column", "type", "--column", "resource", "--column", "task id")

		for _, lock := range parseTable(string(locks.Out.Contents())) {
			if lock[0] == "deployment" && lock[1] == deploymentName {
				fmt.Fprintf(GinkgoWriter, "waiting for deployment lock (task id %s)...\n", lock[2])
				time.Sleep(5 * time.Second)
				continue dance
			}
		}

		break dance
	}
}
