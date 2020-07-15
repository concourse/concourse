package common

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	_ "github.com/lib/pq"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"
	bclient "github.com/concourse/baggageclaim/client"
	"golang.org/x/oauth2"

	"github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	deploymentNamePrefix string
	suiteName            string

	Fly                       = FlyCli{}
	DeploymentName, flyTarget string
	instances                 map[string][]BoshInstance
	jobInstances              map[string][]BoshInstance

	dbInstance *BoshInstance
	DbConn     *sql.DB

	webInstance    *BoshInstance
	AtcExternalURL string
	AtcUsername    string
	AtcPassword    string

	WorkerGardenClient       gclient.Client
	WorkerBaggageclaimClient bclient.Client

	concourseReleaseVersion, bpmReleaseVersion, postgresReleaseVersion string
	vaultReleaseVersion, credhubReleaseVersion, uaaReleaseVersion      string
	stemcellVersion                                                    string
	backupAndRestoreReleaseVersion                                     string

	pipelineName string

	Logger *lagertest.TestLogger

	tmp string
)

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultConsistentlyDuration(time.Minute)
	SetDefaultConsistentlyPollingInterval(time.Second)

	Logger = lagertest.NewTestLogger("test")

	deploymentNamePrefix = os.Getenv("DEPLOYMENT_NAME_PREFIX")
	if deploymentNamePrefix == "" {
		deploymentNamePrefix = "concourse-topgun"
	}

	suiteName = os.Getenv("SUITE")
	if suiteName != "" {
		deploymentNamePrefix += "-" + suiteName
	}

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

	vaultReleaseVersion = os.Getenv("VAULT_RELEASE_VERSION")
	if vaultReleaseVersion == "" {
		vaultReleaseVersion = "latest"
	}

	credhubReleaseVersion = os.Getenv("CREDHUB_RELEASE_VERSION")
	if credhubReleaseVersion == "" {
		credhubReleaseVersion = "latest"
	}

	uaaReleaseVersion = os.Getenv("UAA_RELEASE_VERSION")
	if uaaReleaseVersion == "" {
		uaaReleaseVersion = "latest"
	}

	stemcellVersion = os.Getenv("STEMCELL_VERSION")
	if stemcellVersion == "" {
		stemcellVersion = "latest"
	}
	backupAndRestoreReleaseVersion = os.Getenv("BACKUP_AND_RESTORE_SDK_RELEASE_VERSION")
	if backupAndRestoreReleaseVersion == "" {
		backupAndRestoreReleaseVersion = "latest"
	}

	deploymentNumber := GinkgoParallelNode()

	DeploymentName = fmt.Sprintf("%s-%d", deploymentNamePrefix, deploymentNumber)
	Fly.Target = DeploymentName

	var err error
	tmp, err = ioutil.TempDir("", "topgun-tmp")
	Expect(err).ToNot(HaveOccurred())

	Fly.Home = filepath.Join(tmp, "fly-home")
	err = os.Mkdir(Fly.Home, 0755)
	Expect(err).ToNot(HaveOccurred())

	WaitForDeploymentAndCompileLocks()
	Bosh("delete-deployment", "--force")

	instances = map[string][]BoshInstance{}
	jobInstances = map[string][]BoshInstance{}

	dbInstance = nil
	DbConn = nil
	webInstance = nil
	AtcExternalURL = ""
	AtcUsername = "test"
	AtcPassword = "test"
})

var _ = AfterEach(func() {
	test := CurrentGinkgoTestDescription()
	if test.Failed {
		dir := filepath.Join("logs", fmt.Sprintf("%s.%d", filepath.Base(test.FileName), test.LineNumber))

		err := os.MkdirAll(dir, 0755)
		Expect(err).ToNot(HaveOccurred())

		TimestampedBy("saving logs to " + dir + " due to test failure")
		Bosh("logs", "--dir", dir)
	}

	DeleteAllContainers()

	WaitForDeploymentAndCompileLocks()
	Bosh("delete-deployment")

	Expect(os.RemoveAll(tmp)).To(Succeed())
})

type BoshInstance struct {
	Name  string
	Group string
	ID    string
	IP    string
	DNS   string
}

func StartDeploy(manifest string, args ...string) *gexec.Session {
	WaitForDeploymentAndCompileLocks()

	var modifiedSuiteName string
	if suiteName != "" {
		modifiedSuiteName = "-" + suiteName
	}

	return SpawnBosh(
		append([]string{
			"deploy", manifest,
			"--vars-store", filepath.Join(tmp, DeploymentName+"-vars.yml"),
			"-v", "suite='" + modifiedSuiteName + "'",
			"-v", "deployment_name='" + DeploymentName + "'",
			"-v", "concourse_release_version='" + concourseReleaseVersion + "'",
			"-v", "bpm_release_version='" + bpmReleaseVersion + "'",
			"-v", "postgres_release_version='" + postgresReleaseVersion + "'",
			"-v", "vault_release_version='" + vaultReleaseVersion + "'",
			"-v", "credhub_release_version='" + credhubReleaseVersion + "'",
			"-v", "uaa_release_version='" + uaaReleaseVersion + "'",
			"-v", "backup_and_restore_sdk_release_version='" + backupAndRestoreReleaseVersion + "'",
			"-v", "stemcell_version='" + stemcellVersion + "'",
		}, args...)...,
	)
}

func Deploy(manifest string, args ...string) {
	if DbConn != nil {
		Expect(DbConn.Close()).To(Succeed())
	}

	for {
		deploy := StartDeploy(manifest, args...)
		<-deploy.Exited
		if deploy.ExitCode() != 0 {
			if strings.Contains(string(deploy.Out.Contents()), "Timed out pinging") {
				fmt.Fprintln(GinkgoWriter, "detected ping timeout; trying again...")
				continue
			}

			Fail("deploy failed")
		}

		break
	}

	instances, jobInstances = LoadJobInstances()

	webInstance = JobInstance("web")
	if webInstance != nil {
		AtcExternalURL = fmt.Sprintf("http://%s:8080", webInstance.IP)
		Fly.Login(AtcUsername, AtcPassword, AtcExternalURL)

		WaitForWorkersToBeRunning(len(JobInstances("worker")) + len(JobInstances("other_worker")))

		workers := FlyTable("workers", "-d")
		if len(workers) > 0 {
			worker := workers[0]
			WorkerGardenClient = gclient.New(gconn.New("tcp", worker["garden address"]))
			WorkerBaggageclaimClient = bclient.NewWithHTTPClient(worker["baggageclaim url"], http.DefaultClient)
		} else {
			WorkerGardenClient = nil
			WorkerBaggageclaimClient = nil
		}
	}

	dbInstance = JobInstance("postgres")

	if dbInstance != nil {
		var err error
		DbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", dbInstance.IP))
		Expect(err).ToNot(HaveOccurred())
	}
}

func Instance(name string) *BoshInstance {
	is := instances[name]
	if len(is) == 0 {
		return nil
	}

	return &is[0]
}

func JobInstance(job string) *BoshInstance {
	is := jobInstances[job]
	if len(is) == 0 {
		return nil
	}

	return &is[0]
}

func JobInstances(job string) []BoshInstance {
	return jobInstances[job]
}

func LoadJobInstances() (map[string][]BoshInstance, map[string][]BoshInstance) {
	session := SpawnBosh("instances", "-p", "--dns")
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	output := string(session.Out.Contents())

	instances := map[string][]BoshInstance{}
	jobInstances := map[string][]BoshInstance{}

	lines := strings.Split(output, "\n")
	var instance BoshInstance
	for _, line := range lines {
		instanceMatch := InstanceRow.FindStringSubmatch(line)
		if len(instanceMatch) > 0 {
			group := instanceMatch[1]
			id := instanceMatch[2]

			instance = BoshInstance{
				Name:  group + "/" + id,
				Group: group,
				ID:    id,
				IP:    instanceMatch[4],
				DNS:   instanceMatch[5],
			}

			instances[group] = append(instances[group], instance)

			continue
		}

		jobMatch := JobRow.FindStringSubmatch(line)
		if len(jobMatch) > 0 {
			jobName := jobMatch[2]
			jobInstances[jobName] = append(jobInstances[jobName], instance)
		}
	}

	return instances, jobInstances
}

func Bosh(argv ...string) *gexec.Session {
	session := SpawnBosh(argv...)
	Wait(session)
	return session
}

func SpawnBosh(argv ...string) *gexec.Session {
	return Start(nil, "bosh", append([]string{"-n", "-d", DeploymentName}, argv...)...)
}

func ConcourseClient() concourse.Client {
	token, err := FetchToken(AtcExternalURL, AtcUsername, AtcPassword)
	Expect(err).NotTo(HaveOccurred())

	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	return concourse.NewClient(AtcExternalURL, httpClient, false)
}

func DeleteAllContainers() {
	client := ConcourseClient()
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
					Logger.Error("failed-to-delete-container", err, lager.Data{"handle": container.ID})
				}
			}
		}
	}
}

func WaitForLandedWorker() string {
	return WaitForWorkerInState("landed")
}

func WaitForRunningWorker() string {
	return WaitForWorkerInState("running")
}

func WaitForStalledWorker() string {
	return WaitForWorkerInState("stalled")
}

func WorkerState(name string) string {
	workers := FlyTable("workers")

	for _, w := range workers {
		if w["name"] == name {
			return w["state"]
		}
	}

	return ""
}

func WaitForWorkerInState(desiredStates ...string) string {
	var workerName string

	Eventually(func() string {
		workers := FlyTable("workers")

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

func FlyTable(argv ...string) []map[string]string {
	session := Fly.Start(append([]string{"--print-table-headers"}, argv...)...)
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	result := []map[string]string{}

	var headers []string
	for i, cols := range ParseTable(string(session.Out.Contents())) {
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

func ParseTable(content string) [][]string {
	result := [][]string{}

	var expectedColumns int
	rows := strings.Split(content, "\n")
	for i, row := range rows {
		if row == "" {
			continue
		}

		columns := SplitTableColumns(row)
		if i == 0 {
			expectedColumns = len(columns)
		} else {
			Expect(columns).To(HaveLen(expectedColumns))
		}

		result = append(result, columns)
	}

	return result
}

func SplitTableColumns(row string) []string {
	return regexp.MustCompile(`(\s{2,}|\t)`).Split(strings.TrimSpace(row), -1)
}

func WaitForWorkersToBeRunning(expected int) {
	Eventually(func() interface{} {
		workers := FlyTable("workers")

		runningWorkers := []map[string]string{}
		for _, worker := range workers {
			if worker["state"] == "running" {
				runningWorkers = append(runningWorkers, worker)
			}
		}

		return runningWorkers
	}).Should(HaveLen(expected), "expected all workers to be running")
}

func WorkersWithContainers() []string {
	mainTeam := ConcourseClient().Team("main")
	containers, err := mainTeam.ListContainers(map[string]string{})
	Expect(err).NotTo(HaveOccurred())

	usedWorkers := map[string]struct{}{}

	for _, container := range containers {
		usedWorkers[container.WorkerName] = struct{}{}
	}

	var workerNames []string
	for worker := range usedWorkers {
		workerNames = append(workerNames, worker)
	}

	return workerNames
}

func ContainersBy(condition, value string) []string {
	containers := FlyTable("containers")

	var handles []string
	for _, c := range containers {
		if c[condition] == value {
			handles = append(handles, c["handle"])
		}
	}

	return handles
}

func VolumesByResourceType(name string) []string {
	volumes := FlyTable("volumes", "-d")

	var handles []string
	for _, v := range volumes {
		if v["type"] == "resource" && strings.HasPrefix(v["identifier"], "name:"+name) {
			handles = append(handles, v["handle"])
		}
	}

	return handles
}

func WaitForDeploymentAndCompileLocks() {
	cloudConfig := Start(nil, "bosh", "cloud-config")
	<-cloudConfig.Exited
	cc := struct {
		Compilation struct {
			Workers int
		}
	}{}
	yaml.Unmarshal(cloudConfig.Out.Contents(), &cc)
	numCompilationVms := cc.Compilation.Workers
	for {
		locks := Bosh("locks", "--column", "type", "--column", "resource", "--column", "task id")
		isDeploymentLockClaimed := false
		numCompileLocksClaimed := 0

		for _, lock := range ParseTable(string(locks.Out.Contents())) {
			if lock[0] == "deployment" && lock[1] == DeploymentName {
				fmt.Fprintf(GinkgoWriter, "waiting for deployment lock (task id %s)...\n", lock[2])
				isDeploymentLockClaimed = true
			} else if lock[0] == "compile" {
				numCompileLocksClaimed += 1
			}
		}

		if isDeploymentLockClaimed || numCompileLocksClaimed >= numCompilationVms {
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
}

func PgDump() *gexec.Session {
	dump := exec.Command("pg_dump", "-U", "atc", "-h", dbInstance.IP, "atc")
	dump.Env = append(os.Environ(), "PGPASSWORD=dummy-password")
	dump.Stdin = bytes.NewBufferString("dummy-password\n")
	session, err := gexec.Start(dump, nil, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))
	return session
}

var Psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var _ = SynchronizedBeforeSuite(func() []byte {
	return []byte(BuildBinary())
}, func(data []byte) {
	Fly.Bin = string(data)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

var InstanceRow = regexp.MustCompile(`^([^/]+)/([^\s]+)\s+-\s+(\w+)\s+z1\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\s+([^\s]+)\s*`)
var JobRow = regexp.MustCompile(`^([^\s]+)\s+(\w+)\s+(\w+)\s+-\s+-\s+-\s*`)
