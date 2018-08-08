package cli_test

import (
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	cmd "github.com/concourse/atc/db/migration/cli/command"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migration CLI", func() {

	Context("generate migration files", func() {
		var (
			migrationDir  string
			command       *cmd.GenerateCommand
			migrationName = "create_new_table"
			err           error
		)

		BeforeEach(func() {
			migrationDir, err = ioutil.TempDir("", "")
			Expect(err).ToNot(HaveOccurred())

		})

		AfterEach(func() {
			os.RemoveAll(migrationDir)
		})

		Context("sql migrations", func() {

			It("generates up and down migration files", func() {
				command = cmd.NewGenerateCommand(migrationDir, migrationName, "sql")
				sqlMigrationPattern := "^(\\d+)_(.*).(down|up).sql$"

				err := command.GenerateSQLMigration()
				Expect(err).ToNot(HaveOccurred())

				ExpectGeneratedFilesToMatchSpecification(migrationDir, sqlMigrationPattern,
					migrationName, func(migrationID string, actualFileContents string) {
						Expect(actualFileContents).To(BeEmpty())
					})
			})
		})

		Context("go migration", func() {
			It("generates up and down migration files", func() {
				command = cmd.NewGenerateCommand(migrationDir, migrationName, "go")
				goMigrationPattern := "^(\\d+)_(.*).(down|up).go$"

				err := command.GenerateGoMigration()
				Expect(err).ToNot(HaveOccurred())

				ExpectGeneratedFilesToMatchSpecification(migrationDir, goMigrationPattern,
					migrationName, func(migrationID string, actualFileContents string) {
						lines := strings.Split(actualFileContents, "\n")
						Expect(lines).To(HaveLen(6))

						Expect(lines[0]).To(Equal("package migrations"))
						Expect(lines[1]).To(Equal(""))
						Expect(lines[2]).To(HavePrefix("//"))
						Expect(lines[3]).To(MatchRegexp("^func (Up|Down)_%s\\(\\) error {", migrationID))
						Expect(lines[4]).To(ContainSubstring("return nil"))
						Expect(lines[5]).To(Equal("}"))
					})
			})

		})
	})
})

func ExpectGeneratedFilesToMatchSpecification(migrationDir, fileNamePattern, migrationName string,
	checkContents func(migrationID string, actualFileContents string)) {

	files, err := ioutil.ReadDir(migrationDir)
	Expect(err).ToNot(HaveOccurred())
	var migrationFilesCount = 0
	regex := regexp.MustCompile(fileNamePattern)
	for _, migrationFile := range files {
		var matches []string
		migrationFileName := migrationFile.Name()
		if regex.MatchString(migrationFileName) {
			matches = regex.FindStringSubmatch(migrationFileName)

			Expect(matches).To(HaveLen(4))
			Expect(matches[2]).To(Equal(migrationName))

			fileContents, err := ioutil.ReadFile(path.Join(migrationDir, migrationFileName))
			Expect(err).ToNot(HaveOccurred())
			checkContents(matches[1], string(fileContents))
			migrationFilesCount++
		}
	}
	Expect(migrationFilesCount).To(Equal(2))
}
