package integration_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = XDescribe("ATC 3.13 Team Migration Test", func() {
	var (
		atcProcess ifrit.Process
		atcURL     string

		oldMigrationVersion = 1524079655

		runner ifrit.Runner

		username string
		password string
	)

	BeforeEach(func() {
		username = randomString()
		password = randomString()

		atcURL = fmt.Sprintf("http://localhost:%v", cmd.BindPort)

		cmd.Auth.MainTeamFlags.LocalUsers = []string{username}
		cmd.Auth.AuthFlags.LocalUsers = map[string]string{
			username: password,
		}

		var err error
		runner, err = cmd.Runner([]string{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		atcProcess.Signal(os.Interrupt)
		<-atcProcess.Wait()
	})

	Context("migrations", func() {
		BeforeEach(func() {
			db := postgresRunner.OpenDBAtVersion(oldMigrationVersion)
			defer db.Close()

			auth := fmt.Sprintf(`{"basicauth":{"username":"%s", "password":"%s"}}`, username, password)
			rows, err := db.Query(`UPDATE teams SET auth=$1 WHERE name='main'`, auth)
			rows.Close()

			Expect(err).ToNot(HaveOccurred())
			Expect(db.Close()).ToNot(HaveOccurred())
		})

		It("Successfully migrates", func() {
			atcProcess = ifrit.Invoke(runner)
			Eventually(func() error {
				_, err := http.Get(atcURL + "/api/v1/info")
				return err
			}, 20*time.Second).ShouldNot(HaveOccurred())

			db := postgresRunner.OpenDB()
			rows, err := db.Query(`SELECT auth FROM teams`)

			defer db.Close()
			defer rows.Close()

			Expect(err).ToNot(HaveOccurred())

			var auth string
			for rows.Next() {
				rows.Scan(&auth)
			}
			Expect(auth).To(ContainSubstring(username))
		})
	})
})

var characterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%=")

func randomString() string {
	b := make([]rune, 50)
	for i := range b {
		b[i] = characterRunes[rand.Intn(len(characterRunes))]
	}
	return string(b)
}
