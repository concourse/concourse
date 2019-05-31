package topgun

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/onsi/gomega/gexec"
	"golang.org/x/oauth2"

	. "github.com/onsi/gomega"
)

type Fly struct {
	Bin    string
	Target string
	Home   string
}

type Container struct {
	Type  string `json:"type"`
	State string `json:"state"`
	Id    string `json:"id"`
}

type Worker struct {
	Name            string `json:"name"`
	State           string `json:"state"`
	GardenAddress   string `json:"addr"`
	BaggageclaimUrl string `json:"baggageclaim_url"`
	Team            string `json:"team"`
}

type Pipeline struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Paused   bool   `json:"paused"`
	Public   bool   `json:"public"`
	TeamName string `json:"team_name"`
}

type Version struct {
	ID      int               `json:"id"`
	Version map[string]string `json:"version"`
	Enabled bool              `json:"enabled"`
}

func (f *Fly) Login(user, password, endpoint string) {
	Eventually(func() *gexec.Session {
		sess := f.Start(
			"login",
			"-c", endpoint,
			"-u", user,
			"-p", password,
		)

		<-sess.Exited
		return sess
	}, 2*time.Minute, 10*time.Second).
		Should(gexec.Exit(0), "Fly should have been able to log in")
}

func (f *Fly) Run(argv ...string) {
	Wait(f.Start(argv...))
}

func (f *Fly) Start(argv ...string) *gexec.Session {
	return Start([]string{"HOME=" + f.Home}, f.Bin, append([]string{"-t", f.Target}, argv...)...)
}

func (f *Fly) StartWithEnv(env []string, argv ...string) *gexec.Session {
	return Start(append([]string{"HOME=" + f.Home}, env...), f.Bin, append([]string{"-t", f.Target}, argv...)...)
}

func (f *Fly) SpawnInteractive(stdin io.Reader, argv ...string) *gexec.Session {
	return SpawnInteractive(stdin, []string{"HOME=" + f.Home}, f.Bin, append([]string{"-t", f.Target}, argv...)...)
}

func (f *Fly) GetContainers() []Container {
	var containers = []Container{}

	sess := f.Start("containers", "--json")
	<-sess.Exited
	Expect(sess.ExitCode()).To(BeZero())

	err := json.Unmarshal(sess.Out.Contents(), &containers)
	Expect(err).ToNot(HaveOccurred())

	return containers
}

func (f *Fly) GetWorkers() []Worker {
	var workers = []Worker{}

	sess := f.Start("workers", "--json")
	<-sess.Exited
	Expect(sess.ExitCode()).To(BeZero())

	err := json.Unmarshal(sess.Out.Contents(), &workers)
	Expect(err).ToNot(HaveOccurred())

	return workers
}

func (f *Fly) GetPipelines() []Pipeline {
	var pipelines = []Pipeline{}

	sess := f.Start("pipelines", "--json")
	<-sess.Exited
	Expect(sess.ExitCode()).To(BeZero())

	err := json.Unmarshal(sess.Out.Contents(), &pipelines)
	Expect(err).ToNot(HaveOccurred())

	return pipelines
}

func (f *Fly) GetVersions(pipeline string, resource string) []Version {
	var versions = []Version{}

	sess := f.Start("resource-versions", "-r", pipeline+"/"+resource, "--json")
	<-sess.Exited
	Expect(sess.ExitCode()).To(BeZero())

	err := json.Unmarshal(sess.Out.Contents(), &versions)
	Expect(err).ToNot(HaveOccurred())

	return versions
}

func (f *Fly) GetUserRole(teamName string) []string {

	type RoleInfo struct {
		Teams map[string][]string `json:"teams"`
	}
	var teamsInfo RoleInfo = RoleInfo{}

	sess := f.Start("userinfo", "--json")
	<-sess.Exited
	Expect(sess.ExitCode()).To(BeZero())

	err := json.Unmarshal(sess.Out.Contents(), &teamsInfo)
	Expect(err).ToNot(HaveOccurred())

	return teamsInfo.Teams[teamName]

}

func BuildBinary() string {
	flyBinPath, err := gexec.Build("github.com/concourse/concourse/v5/fly")
	Expect(err).ToNot(HaveOccurred())

	return flyBinPath
}

func RequestCredsInfo(webUrl, token string) ([]byte, error) {
	request, err := http.NewRequest("GET", webUrl+"/api/v1/info/creds", nil)
	Expect(err).ToNot(HaveOccurred())

	reqHeader := http.Header{}
	reqHeader.Set("Authorization", "Bearer "+token)

	request.Header = reqHeader

	client := &http.Client{}
	resp, err := client.Do(request)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(200))

	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return body, err
}

func FetchToken(webURL, username, password string) (*oauth2.Token, error) {
	oauth2Config := oauth2.Config{
		ClientID:     "fly",
		ClientSecret: "Zmx5",
		Endpoint:     oauth2.Endpoint{TokenURL: webURL + "/sky/token"},
		Scopes:       []string{"openid", "profile", "email", "federated:id"},
	}

	return oauth2Config.PasswordCredentialsToken(context.Background(), username, password)
}
