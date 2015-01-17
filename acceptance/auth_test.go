package acceptance_test

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/concourse/atc/db"
)

var _ = Describe("Auth", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		postgresRunner.CreateTestDB()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		sqlDB = db.NewSQL(logger, dbConn, dbListener)

		atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
		Ω(err).ShouldNot(HaveOccurred())

		atcCommand := exec.Command(
			atcBin,
			"-httpUsername", "admin",
			"-httpHashedPassword", "$2a$04$Cl3vCfrp01EM9NGekxL59uPusP/hBIM3toCkCuECK3saCbOAyrg/O", // "password"
			"-templates", filepath.Join("..", "web", "templates"),
			"-public", filepath.Join("..", "web", "public"),
			"-sqlDataSource", postgresRunner.DataSourceName(),
		)
		atcRunner := ginkgomon.New(ginkgomon.Config{
			Command:       atcCommand,
			Name:          "atc",
			StartCheck:    "atc.listening",
			AnsiColorCode: "32m",
		})
		atcProcess = ginkgomon.Invoke(atcRunner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Ω(dbConn.Close()).Should(Succeed())
		Ω(dbListener.Close()).Should(Succeed())

		postgresRunner.DropTestDB()
	})

	It("can reach the page", func() {
		request, err := http.NewRequest("GET", "http://127.0.0.1:8080", nil)

		resp, err := http.DefaultClient.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(resp.StatusCode).Should(Equal(http.StatusUnauthorized))

		request.SetBasicAuth("admin", "password")
		resp, err = http.DefaultClient.Do(request)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(resp.StatusCode).Should(Equal(http.StatusOK))
	})
})
