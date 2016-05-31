package fly_test

import (
	"os/exec"

	"github.com/concourse/fly/rc"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

const bogusTarget = "bogus-target"

var _ = Describe("the quality of being unauthenticated", func() {
	BeforeEach(func() {
		noAuth, _, _, err := helpers.GetAuthMethods(atcURL)
		Expect(err).ToNot(HaveOccurred())
		if noAuth {
			Skip("No auth methods enabled; skipping unauthenticated tests")
		}

		err = rc.SaveTarget(bogusTarget, atcURL, false, "main", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	DescribeTable("trying to run commands when unauthenticated",
		func(passedArgs ...string) {
			args := append([]string{"-t", bogusTarget}, passedArgs...)
			fly := exec.Command(flyBin, args...)
			session := helpers.StartFly(fly)

			<-session.Exited
			Expect(session.ExitCode()).To(Equal(1))

			Expect(session.Err).To(gbytes.Say("not authorized"))
		},

		Entry("get-pipeline", "get-pipeline", "-p", "john"),
		Entry("set-pipeline", "set-pipeline", "-p", "john", "-c", "../fixtures/simple-pipeline.yml"),
		Entry("pause-pipeline", "pause-pipeline", "-p", "john"),
		Entry("unpause-pipeline", "unpause-pipeline", "-p", "john"),
		Entry("checklist", "checklist", "-p", "john"),
		Entry("containers", "containers"),
		Entry("volumes", "volumes"),
		Entry("workers", "workers"),
		Entry("execute", "execute", "-c", "../fixtures/simple-task.yml"),
		Entry("watch", "watch"),
		Entry("hijack", "hijack"),
	)

	DescribeTable("trying to run commands that require confirmation when unauthenticated",
		func(passedArgs ...string) {
			args := append([]string{"-t", bogusTarget}, passedArgs...)
			fly := exec.Command(flyBin, args...)
			session := helpers.StartFly(fly)

			<-session.Exited
			Expect(session.ExitCode()).To(Equal(1))

			Expect(session.Err).To(gbytes.Say("not authorized"))
		},

		Entry("destroy-pipeline", "destroy-pipeline", "-p", "john", "-n"),
	)
})
