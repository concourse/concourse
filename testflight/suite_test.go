package testflight_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/go-concourse/concourse"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/onsi/gomega/gexec"
)

const testflightFlyTarget = "tf"
const testflightGuestFlyTarget = "tf-guest"
const adminFlyTarget = "tf-admin"

const pipelinePrefix = "tf-pipeline"

const teamName = "testflight"
const adminTeamName = "main"
const guestTeamName = "testflight-guest"

var flyTarget string

type suiteConfig struct {
	FlyBin           string `json:"fly"`
	ATCURL           string `json:"atc_url"`
	ATCUsername      string `json:"atc_username"`
	ATCPassword      string `json:"atc_password"`
	ATCGuestUsername string `json:"atc_guest_username"`
	ATCGuestPassword string `json:"atc_guest_password"`
	DownloadCLI      bool   `json:"download_cli"`
}

var (
	config = suiteConfig{
		ATCURL:           "http://localhost:8080",
		ATCUsername:      "test",
		ATCPassword:      "test",
		ATCGuestUsername: "guest",
		ATCGuestPassword: "guest",
	}

	pipelineName string
	tmp          string
)

func TestTestflight(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestFlight Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	atcURL := os.Getenv("ATC_URL")
	if atcURL != "" {
		config.ATCURL = atcURL
	}

	var err error
	downloadCLI := os.Getenv("DOWNLOAD_CLI")
	if downloadCLI != "" {
		config.DownloadCLI, err = strconv.ParseBool(downloadCLI)
		Expect(err).ToNot(HaveOccurred())
	}

	if config.DownloadCLI {
		config.FlyBin, err = downloadFly(config.ATCURL)
		Expect(err).ToNot(HaveOccurred())
	} else {
		config.FlyBin, err = gexec.Build("github.com/concourse/concourse/fly")
		Expect(err).ToNot(HaveOccurred())
	}

	atcUsername := os.Getenv("ATC_USERNAME")
	if atcUsername != "" {
		config.ATCUsername = atcUsername
	}

	atcPassword := os.Getenv("ATC_PASSWORD")
	if atcPassword != "" {
		config.ATCPassword = atcPassword
	}

	atcGuestUsername := os.Getenv("ATC_GUEST_USERNAME")
	if atcGuestUsername != "" {
		config.ATCGuestUsername = atcGuestUsername
	}

	atcGuestPassword := os.Getenv("ATC_GUEST_PASSWORD")
	if atcGuestPassword != "" {
		config.ATCGuestPassword = atcGuestPassword
	}

	payload, err := json.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() *gexec.Session {
		return flyLogin(adminFlyTarget, adminTeamName, config.ATCUsername, config.ATCPassword)
	}, 2*time.Minute, time.Second).Should(gexec.Exit(0))

	fly("-t", adminFlyTarget, "set-team", "--non-interactive", "-n", guestTeamName, "--local-user", config.ATCGuestUsername)
	flyLogin(testflightGuestFlyTarget, guestTeamName, config.ATCGuestUsername, config.ATCGuestPassword)

	fly("-t", adminFlyTarget, "set-team", "--non-interactive", "-n", teamName, "--local-user", config.ATCUsername)
	flyLogin(testflightFlyTarget, teamName, config.ATCUsername, config.ATCPassword)

	for _, target := range []string{adminFlyTarget, testflightFlyTarget, testflightGuestFlyTarget} {
		for _, ps := range flyTable("-t", target, "pipelines") {
			name := ps["name"]
			if strings.HasPrefix(name, pipelinePrefix) {
				fly("-t", target, "destroy-pipeline", "-n", "-p", name)
			}
		}
	}

	return payload
}, func(data []byte) {
	err := json.Unmarshal(data, &config)
	Expect(err).ToNot(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.Remove(config.FlyBin)
})

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultConsistentlyDuration(time.Minute)
	SetDefaultConsistentlyPollingInterval(time.Second)

	var err error
	tmp, err = ioutil.TempDir("", "testflight-tmp")
	Expect(err).ToNot(HaveOccurred())

	flyTarget = testflightFlyTarget

	pipelineName = randomPipelineName()
})

var _ = AfterEach(func() {
	Expect(os.RemoveAll(tmp)).To(Succeed())

	fly("destroy-pipeline", "-n", "-p", pipelineName)
})

func downloadFly(atcUrl string) (string, error) {
	client := concourse.NewClient(atcUrl, http.DefaultClient, false)
	readCloser, _, err := client.GetCLIReader("amd64", runtime.GOOS)
	if err != nil {
		return "", err
	}
	outFile, err := ioutil.TempFile("", "fly")
	if err != nil {
		return "", err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, readCloser)
	if err != nil {
		return "", err
	}
	err = outFile.Chmod(0755)
	if err != nil {
		return "", err
	}
	return outFile.Name(), nil
}

func randomPipelineName() string {
	guid, err := uuid.NewV4()
	Expect(err).ToNot(HaveOccurred())

	return fmt.Sprintf("%s-%d-%s", pipelinePrefix, GinkgoParallelNode(), guid)
}

func fly(argv ...string) *gexec.Session {
	sess := spawnFly(argv...)
	wait(sess, false)
	return sess
}

func flyIn(dir string, argv ...string) *gexec.Session {
	sess := spawnFlyIn(dir, argv...)
	wait(sess, false)
	return sess
}

func flyUnsafe(argv ...string) *gexec.Session {
	sess := spawnFly(argv...)
	wait(sess, true)
	return sess
}

func flyLogin(target, team, username, password string) *gexec.Session {
	sess := spawn(
		config.FlyBin,
		"-t", target,
		"login",
		"-c", config.ATCURL,
		"-n", team,
		"-u", username,
		"-p", password,
	)
	wait(sess, false)
	return sess
}

func spawnFly(argv ...string) *gexec.Session {
	return spawn(config.FlyBin, append([]string{"-t", flyTarget}, argv...)...)
}

func spawnFlyIn(dir string, argv ...string) *gexec.Session {
	return spawnIn(dir, config.FlyBin, append([]string{"-t", flyTarget}, argv...)...)
}

func spawnFlyOpts(opts ...spawnOpt) func(argv ...string) *gexec.Session {
	spawnFn := spawnOpts(opts...)
	return func(argv ...string) *gexec.Session {
		return spawnFn(config.FlyBin, append([]string{"-t", flyTarget}, argv...)...)
	}
}

type spawnOpt func(*exec.Cmd) *exec.Cmd

func withStdin(stdin io.Reader) spawnOpt {
	return func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Stdin = stdin
		return cmd
	}
}

func spawnOpts(opts ...spawnOpt) func(argc string, argv ...string) *gexec.Session {
	return func(argc string, argv ...string) *gexec.Session {
		By("running: " + argc + " " + strings.Join(argv, " "))
		cmd := exec.Command(argc, argv...)
		for _, opt := range opts {
			cmd = opt(cmd)
		}
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		return session
	}
}

func spawn(argc string, argv ...string) *gexec.Session {
	By("running: " + argc + " " + strings.Join(argv, " "))
	cmd := exec.Command(argc, argv...)
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func spawnIn(dir string, argc string, argv ...string) *gexec.Session {
	By("running in " + dir + ": " + argc + " " + strings.Join(argv, " "))
	cmd := exec.Command(argc, argv...)
	cmd.Dir = dir
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	return session
}

func wait(session *gexec.Session, allowNonZero bool) {
	<-session.Exited
	if !allowNonZero {
		Expect(session.ExitCode()).To(Equal(0), "Output: "+string(session.Out.Contents()))
	}
}

var colSplit = regexp.MustCompile(`\s{2,}`)

func flyTable(argv ...string) []map[string]string {
	session := spawnFly(append([]string{"--print-table-headers"}, argv...)...)
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))

	result := []map[string]string{}
	var headers []string

	rows := strings.Split(string(session.Out.Contents()), "\n")
	for i, row := range rows {
		columns := colSplit.Split(strings.TrimSpace(row), -1)

		if i == 0 {
			headers = columns
			continue
		}

		if row == "" {
			continue
		}

		result = append(result, map[string]string{})

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

func setAndUnpausePipeline(config string, args ...string) {
	setPipeline(config, args...)
	fly("unpause-pipeline", "-p", pipelineName)
}

func setPipeline(config string, args ...string) {
	sp := []string{"set-pipeline", "-n", "-p", pipelineName, "-c", config}
	fly(append(sp, args...)...)
}

func inPipeline(thing string) string {
	return pipelineName + "/" + thing
}

func newMockVersion(resourceName string, tag string) string {
	guid, err := uuid.NewV4()
	Expect(err).ToNot(HaveOccurred())

	version := guid.String() + "-" + tag

	fly("check-resource", "-r", inPipeline(resourceName), "-f", "version:"+version)

	return version
}

func waitForBuildAndWatch(jobName string, buildName ...string) *gexec.Session {
	args := []string{"watch", "-j", inPipeline(jobName)}

	if len(buildName) > 0 {
		args = append(args, "-b", buildName[0])
	}

	keepPollingCheck := regexp.MustCompile("job has no builds|build not found|failed to get build")
	for {
		session := spawnFly(args...)
		<-session.Exited

		if session.ExitCode() == 1 {
			output := strings.TrimSpace(string(session.Err.Contents()))
			if keepPollingCheck.MatchString(output) {
				// build hasn't started yet; keep polling
				time.Sleep(time.Second)
				continue
			}
		}

		return session
	}
}

func withFlyTarget(target string, f func()) {
	before := flyTarget
	flyTarget = target
	f()
	flyTarget = before
}
