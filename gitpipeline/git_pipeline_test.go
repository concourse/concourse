package git_pipeline_test

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/concourse/testflight/guidserver"
	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline with git resources", func() {
	It("triggers when the repo changes", func() {
		By("building the initial commit")
		guid1 := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(guid1))

		By("building another commit")
		guid2 := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(guid2))
	})

	It("performs output conditions correctly", func() {
		By("executing the build when a commit is made")
		committedGuid := gitServer.Commit()
		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(committedGuid))

		masterSHA := gitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		By("performing on: [success] outputs on success")
		Eventually(func() string {
			return successGitServer.RevParse("success")
		}, 30*time.Second, 1*time.Second).Should(Equal(masterSHA))

		By("performing on: [failure] outputs on failure")
		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}, 30*time.Second, 1*time.Second).Should(Equal(masterSHA))

		By("not performing on: [success] outputs on failure")
		Consistently(func() string {
			return noUpdateGitServer.RevParse("no-update")
		}, 10*time.Second, 1*time.Second).Should(BeEmpty())

		By("always performs ensure on: [success]")
		Eventually(func() string {
			return ensureSuccessGitServer.RevParse("ensure-success")
		}, 30*time.Second, 1*time.Second).Should(Equal(masterSHA))

		By("always performs ensure on: [failure]")
		Eventually(func() string {
			return ensureFailureGitServer.RevParse("ensure-failure")
		}, 30*time.Second, 1*time.Second).Should(Equal(masterSHA))
	})

	It("performs build matrixes correctly", func() {
		By("executing the build when a commit is made")
		committedGuid := gitServer.Commit()

		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement("passing-unit-1/file passing-unit-2/file " + committedGuid))

		Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement("failed " + committedGuid))
	})

	Describe("try", func() {
		It("proceeds with build plan if wrapped task fails", func() {
			By("executing the build when a commit is made")
			committedGuid := gitServer.Commit()
			Eventually(guidserver.ReportingGuids, 5*time.Minute, 10*time.Second).Should(ContainElement(committedGuid))

			fly := exec.Command(
				flyBin,
				"-t", atcURL,
				"watch",
				"-p", "pipeline-name",
				"-j", "try-job",
			)

			flyS := start(fly)

			Eventually(flyS, 30*time.Second).Should(gbytes.Say("initializing"))

			Eventually(flyS, 30*time.Second).Should(gbytes.Say("passing-task succeeded"))

			Eventually(flyS, 10*time.Second).Should(gexec.Exit(0))
		})
	})
})

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
	Ω(err).ShouldNot(HaveOccurred())

	return session
}
