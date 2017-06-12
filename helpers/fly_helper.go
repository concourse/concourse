package helpers

import (
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mgutz/ansi"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var TargetedConcourse = "testflight"

type FlyHelper struct {
	Path string
}

func (h *FlyHelper) DestroyPipeline(pipelineName string) {
	destroyCmd := exec.Command(
		h.Path,
		"-t", TargetedConcourse,
		"destroy-pipeline",
		"-p", pipelineName,
		"-n",
	)

	destroy, err := gexec.Start(destroyCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-destroy.Exited

	if destroy.ExitCode() == 1 {
		if strings.Contains(string(destroy.Err.Contents()), "does not exist") {
			return
		}
	}

	Expect(destroy).To(gexec.Exit(0))
}

func (h *FlyHelper) RenamePipeline(oldName string, newName string) {
	renameCmd := exec.Command(
		h.Path,
		"-t", TargetedConcourse,
		"rename-pipeline",
		"-o", oldName,
		"-n", newName,
	)

	rename, err := gexec.Start(renameCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-rename.Exited
	Expect(rename).To(gexec.Exit(0))
}

func (h *FlyHelper) ConfigurePipeline(pipelineName string, argv ...string) {
	h.DestroyPipeline(pipelineName)

	h.ReconfigurePipeline(pipelineName, argv...)
}

func (h *FlyHelper) ReconfigurePipeline(pipelineName string, argv ...string) {
	args := append([]string{
		"-t", TargetedConcourse,
		"set-pipeline",
		"-p", pipelineName,
		"-n",
	}, argv...)

	configureCmd := exec.Command(h.Path, args...)

	configure, err := gexec.Start(configureCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-configure.Exited
	Expect(configure.ExitCode()).To(Equal(0))

	h.UnpausePipeline(pipelineName)
}

func (h *FlyHelper) PausePipeline(pipelineName string) {
	pauseCmd := exec.Command(h.Path, "-t", TargetedConcourse, "pause-pipeline", "-p", pipelineName)

	configure, err := gexec.Start(pauseCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-configure.Exited
	Expect(configure.ExitCode()).To(Equal(0))

	Expect(configure).To(gbytes.Say("paused '%s'", pipelineName))
}

func (h *FlyHelper) UnpausePipeline(pipelineName string) {
	unpauseCmd := exec.Command(h.Path, "-t", TargetedConcourse, "unpause-pipeline", "-p", pipelineName)

	configure, err := gexec.Start(unpauseCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-configure.Exited
	Expect(configure.ExitCode()).To(Equal(0))

	Expect(configure).To(gbytes.Say("unpaused '%s'", pipelineName))
}

func (h *FlyHelper) Watch(pipelineName string, jobName string, buildName ...string) *gexec.Session {
	args := []string{
		"-t", TargetedConcourse,
		"watch",
		"-j", pipelineName + "/" + jobName,
	}

	if len(buildName) > 0 {
		args = append(args, "-b", buildName[0])
	}

	keepPollingCheck := regexp.MustCompile("job has no builds|build not found|failed to get build")
	for {
		session := start(exec.Command(h.Path, args...))

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

func (h *FlyHelper) TriggerJob(pipelineName string, jobName string) *gexec.Session {
	return h.TriggerPipelineJob(pipelineName, jobName)
}

func (h *FlyHelper) AbortBuild(pipelineName string, jobName string, build int) {
	sess := start(exec.Command(
		h.Path,
		"-t",
		TargetedConcourse,
		"abort-build",
		"-j", pipelineName+"/"+jobName,
		"-b", strconv.Itoa(build),
	))
	<-sess.Exited
	Expect(sess).To(gexec.Exit(0))
}

func (h *FlyHelper) TriggerPipelineJob(pipeline string, jobName string) *gexec.Session {
	return start(exec.Command(
		h.Path,
		"-t",
		TargetedConcourse,
		"trigger-job",
		"-j", pipeline+"/"+jobName,
		"-w",
	))
}

func (h *FlyHelper) Execute(dir string, argv ...string) *gexec.Session {
	args := append([]string{
		"-t",
		TargetedConcourse,
		"execute",
	}, argv...)

	command := exec.Command(
		h.Path,
		args...,
	)
	command.Dir = dir

	return start(command)
}

func (h *FlyHelper) Hijack(argv ...string) *gexec.Session {
	args := append([]string{
		"-t",
		TargetedConcourse,
		"hijack",
	}, argv...)

	command := exec.Command(
		h.Path,
		args...,
	)

	return start(command)
}

func (h *FlyHelper) HijackInteractive(argv ...string) (*gexec.Session, io.Writer) {
	args := append([]string{
		"-t",
		TargetedConcourse,
		"hijack",
	}, argv...)

	command := exec.Command(
		h.Path,
		args...,
	)

	stdin, err := command.StdinPipe()
	Expect(err).ToNot(HaveOccurred())

	return start(command), stdin
}

func start(cmd *exec.Cmd) *gexec.Session {
	session, err := gexec.Start(
		cmd,
		gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[fly]", "blue")),
			GinkgoWriter,
		),
		gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[fly]", "blue")),
			GinkgoWriter,
		),
	)
	Expect(err).NotTo(HaveOccurred())

	return session
}

func (h *FlyHelper) Table(argv ...string) []map[string]string {
	session := start(exec.Command(
		h.Path,
		append([]string{"-t", TargetedConcourse, "--print-table-headers"}, argv...)...,
	))
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
