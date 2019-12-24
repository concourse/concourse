package topgun_test

import (
	"bufio"
	"os"
	"strings"

	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("An ATC with syslog draining set", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml",
			"-o", "operations/syslog_configurations.yml",
			"-v", "syslog.address=localhost:8081",
			"-v", "syslog.hostname=atc1",
			"-v", "syslog.transport=tcp",
			"-v", "syslog.drain_interval=1s",
		)
	})

	It("sends the build logs to the syslog server.", func() {
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/secrets.yml", "-p", "syslog-pipeline")

		Fly.Run("unpause-pipeline", "-p", "syslog-pipeline")
		buildSession := Fly.Start("trigger-job", "-w", "-j", "syslog-pipeline/simple-job")

		<-buildSession.Exited
		Expect(buildSession.ExitCode()).To(Equal(0))

		Eventually(func()(bool, error){
			Bosh("scp", "web/0:/var/vcap/store/syslog_storer/syslog.log", "/tmp/syslog.log")
			return checkContent("/tmp/syslog.log", "shhhh")
		}).Should(BeTrue())
	})
})

func checkContent(path string, stringToCheck string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)

	line := 1
	// https://golang.org/pkg/bufio/#Scanner.Scan
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), stringToCheck) {
			return true, nil
		}

		line++
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}
