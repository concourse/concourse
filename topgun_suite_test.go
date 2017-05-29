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
	jobIPs                    map[string][]string

	dbIP                    string
	atcIP, atcExternalURL   string
	atcIP2, atcExternalURL2 string
	gitServerIP             string

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

	stemcellVersion = os.Getenv("STEMCELL_VERSION")
	if stemcellVersion == "" {
		stemcellVersion = "latest"
	}

	deploymentNumber := GinkgoParallelNode() + (networkOffset * 4)

	deploymentName = fmt.Sprintf("concourse-topgun-%d", deploymentNumber)
	flyTarget = deploymentName

	bosh("delete-deployment")

	jobIPs = map[string][]string{}
})

var _ = AfterEach(func() {
	boshLogs.Signal(os.Interrupt)
	<-boshLogs.Exited
	boshLogs = nil

	deleteAllContainers()

	bosh("delete-deployment")
})

func StartDeploy(manifest string, operations ...string) *gexec.Session {
	opFlags := []string{}
	for _, op := range operations {
		opFlags = append(opFlags, fmt.Sprintf("-o=%s", op))
	}

	return spawnBosh(
		append([]string{
			"deploy", manifest,
			"-v", "deployment-name=" + deploymentName,
			"-v", "concourse-release-version=" + concourseReleaseVersion,
			"-v", "garden-runc-release-version=" + gardenRuncReleaseVersion,

			// 3363.10 becomes 3363.1 as it's floating point; quotes prevent that
			"-v", "stemcell-version='" + stemcellVersion + "'",
		}, opFlags...)...,
	)
}

func Deploy(manifest string, operations ...string) {
	wait(StartDeploy(manifest, operations...))

	jobIPs = loadJobIPs()

	atcIPs := JobIPs("atc")
	atcIP = atcIPs[0]
	if len(atcIPs) > 1 {
		atcIP2 = atcIPs[1]
	} else {
		atcIP2 = ""
	}

	dbIP = JobIP("postgresql")
	gitServerIP = JobIP("git_server")

	atcExternalURL = fmt.Sprintf("http://%s:8080", atcIP)
	atcExternalURL2 = fmt.Sprintf("http://%s:8080", atcIP2)

	// give some time for atc to bootstrap (run migrations, etc)
	Eventually(func() int {
		flySession := spawnFly("login", "-c", atcExternalURL)
		<-flySession.Exited
		return flySession.ExitCode()
	}, 2*time.Minute).Should(Equal(0))

	boshLogs = spawnBosh("logs", "-f")
}

func JobIP(instance string) string {
	ips := jobIPs[instance]
	if len(ips) == 0 {
		return ""
	}

	return ips[0]
}

func JobIPs(instance string) []string {
	return jobIPs[instance]
}

var instanceRow = regexp.MustCompile(`^([^/]+)/([^\s]+)\s+-\s+(\w+)\s+z1\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\s*$`)
var jobRow = regexp.MustCompile(`^([^/]+)/([^\s]+)\s+(\w+)\s+(\w+)\s+-\s+-\s*$`)

func loadJobIPs() map[string][]string {
	session := spawnBosh("instances", "-p")
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	output := string(session.Out.Contents())

	jobIPs := map[string][]string{}

	lines := strings.Split(output, "\n")
	var instanceIP string
	for _, line := range lines {
		instanceMatch := instanceRow.FindStringSubmatch(line)
		if len(instanceMatch) > 0 {
			instanceIP = instanceMatch[4]
			continue
		}

		jobMatch := jobRow.FindStringSubmatch(line)
		if len(jobMatch) > 0 {
			jobName := jobMatch[3]
			jobIPs[jobName] = append(jobIPs[jobName], instanceIP)
		}
	}

	return jobIPs
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
