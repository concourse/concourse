package git_pipeline_test

import (
	"fmt"
	"os/exec"

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
		Eventually(guidserver.ReportingGuids).Should(ContainElement(guid1))

		By("building another commit")
		guid2 := gitServer.Commit()
		Eventually(guidserver.ReportingGuids).Should(ContainElement(guid2))
	})

	It("performs output conditions correctly", func() {
		By("executing the build when a commit is made")
		committedGuid := gitServer.Commit()
		Eventually(guidserver.ReportingGuids).Should(ContainElement(committedGuid))

		masterSHA := gitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		By("performing on_success outputs on success")
		Eventually(func() string {
			return successGitServer.RevParse("success")
		}).Should(Equal(masterSHA))

		By("performing on_failure steps on failure")
		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}).Should(Equal(masterSHA))

		By("not performing on_success steps on failure or on_failure steps on success")
		Consistently(func() string {
			return noUpdateGitServer.RevParse("no-update")
		}).Should(BeEmpty())

		By("performing ensure steps on success")
		Eventually(func() string {
			return ensureSuccessGitServer.RevParse("ensure-success")
		}).Should(Equal(masterSHA))

		By("peforming ensure steps on failure")
		Eventually(func() string {
			return ensureFailureGitServer.RevParse("ensure-failure")
		}).Should(Equal(masterSHA))
	})

	It("performs build matrixes correctly", func() {
		By("executing the build when a commit is made")
		committedGuid := gitServer.Commit()

		Eventually(guidserver.ReportingGuids).Should(ContainElement("passing-unit-1/file passing-unit-2/file " + committedGuid))

		Eventually(guidserver.ReportingGuids).Should(ContainElement("failed " + committedGuid))
	})

	Describe("try", func() {
		It("proceeds with build plan if wrapped task fails", func() {
			committedGuid := gitServer.Commit()
			Eventually(guidserver.ReportingGuids).Should(ContainElement(committedGuid))

			fly := exec.Command(
				flyBin,
				"-t", atcURL,
				"watch",
				"-p", "pipeline-name",
				"-j", "try-job",
			)

			flyS := start(fly)

			Eventually(flyS).Should(gbytes.Say("initializing"))
			Eventually(flyS).Should(gbytes.Say("passing-task succeeded"))
			Eventually(flyS).Should(gexec.Exit(0))
		})
	})

	Describe("a job with a failing task", func() {
		It("causes the job to fail", func() {
			committedGuid := gitServer.Commit()
			Eventually(guidserver.ReportingGuids).Should(ContainElement(committedGuid))

			fly := exec.Command(
				flyBin,
				"-t", atcURL,
				"watch",
				"-p", "pipeline-name",
				"-j", "failing-job",
			)

			flyS := start(fly)

			Eventually(flyS).Should(gbytes.Say("initializing"))
			Eventually(flyS).Should(gbytes.Say("failed"))
			Eventually(flyS).Should(gexec.Exit(1))
		})
	})

	Describe("a timeout on a task", func() {
		It("does not effect the task if it finishes before the timeout", func() {
			committedGuid := gitServer.Commit()
			Eventually(guidserver.ReportingGuids).Should(ContainElement(committedGuid))

			fly := exec.Command(
				flyBin,
				"-t", atcURL,
				"watch",
				"-p", "pipeline-name",
				"-j", "duration-successful-job",
			)

			flyS := start(fly)

			Eventually(flyS).Should(gbytes.Say("initializing"))
			Eventually(flyS).Should(gbytes.Say("passing-task succeeded"))
			Eventually(flyS).Should(gexec.Exit(0))
		})

		It("interrupts the task if it takes longer than the timeout", func() {
			committedGuid := gitServer.Commit()
			Eventually(guidserver.ReportingGuids).Should(ContainElement(committedGuid))

			fly := exec.Command(
				flyBin,
				"-t", atcURL,
				"watch",
				"-p", "pipeline-name",
				"-j", "duration-fail-job",
			)

			flyS := start(fly)

			Eventually(flyS).Should(gbytes.Say("initializing"))
			Eventually(flyS).Should(gbytes.Say("interrupted"))
			Eventually(flyS).Should(gexec.Exit(1))
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
