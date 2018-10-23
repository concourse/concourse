package topgun_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type GitRepo interface {
	Cleanup()
	CommitAndPush()
}

type gitRepo struct {
	remoteURI string
	dir       string
}

func NewGitRepo(uri string) GitRepo {
	dir, err := ioutil.TempDir("", "git-repo")
	Expect(err).NotTo(HaveOccurred())

	c := exec.Command("sh", "-c",
		fmt.Sprintf(`
      cd %s
      git init
      git config user.email dummy@example.com
      git config user.name "Dummy User"
      git remote add origin %s
    `, dir, uri),
	)
	setupProcess, err := gexec.Start(c, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	<-setupProcess.Exited
	Expect(setupProcess.ExitCode()).To(Equal(0))

	return &gitRepo{
		dir:       dir,
		remoteURI: uri,
	}
}

func (r *gitRepo) Cleanup() {
	Expect(os.RemoveAll(r.dir)).To(Succeed())
}

func (r *gitRepo) CommitAndPush() {
	guid, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())

	c := exec.Command("sh", "-c",
		fmt.Sprintf(`
      cd %s
      touch %s
      git add -A
      git commit -m "%s"
      git push -u origin master
    `, r.dir, guid, guid),
	)

	commitAndPushSession, err := gexec.Start(c, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-commitAndPushSession.Exited
	Expect(commitAndPushSession.ExitCode()).To(Equal(0))
}
