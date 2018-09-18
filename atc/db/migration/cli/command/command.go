package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/template"
	"time"
)

var defaultMigrationDir = "migrations/"

type MigrationType string

const (
	SQL MigrationType = "sql"
	Go  MigrationType = "go"
)

type MigrationCommand struct {
	GenerateCommand GenerateCommand `command:"generate"`
}

type GenerateCommand struct {
	MigrationDirectory string        `short:"d" long:"directory" default:"migrations" description:"The directory to which the migration files should be written"`
	MigrationName      string        `short:"n" long:"name" description:"The name of the migration"`
	Type               MigrationType `short:"t" long:"type" description:"The file type of the migration"`
}

func NewGenerateCommand(migrationDir string, migrationName string, migrationType MigrationType) *GenerateCommand {
	if migrationDir == "" {
		migrationDir = defaultMigrationDir
	}

	return &GenerateCommand{
		migrationDir,
		migrationName,
		migrationType,
	}
}

func (c *GenerateCommand) Execute(args []string) error {
	if c.Type == SQL {
		return c.GenerateSQLMigration()
	} else if c.Type == Go {
		return c.GenerateGoMigration()
	}
	return fmt.Errorf("unsupported migration type %s. Supported types include %s and %s", c.Type, SQL, Go)
}

func (c *GenerateCommand) GenerateSQLMigration() error {
	currentTime := time.Now().Unix()
	fileNameFormat := "%d_%s.%s.sql"

	upMigrationFileName := fmt.Sprintf(fileNameFormat, currentTime, c.MigrationName, "up")
	downMigrationFileName := fmt.Sprintf(fileNameFormat, currentTime, c.MigrationName, "down")

	contents := ""

	err := ioutil.WriteFile(path.Join(c.MigrationDirectory, upMigrationFileName), []byte(contents), os.ModePerm)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(c.MigrationDirectory, downMigrationFileName), []byte(contents), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

type migrationInfo struct {
	MigrationId int64
	Direction   string
}

const goMigrationTemplate = `package migrations

// implement the migration in this function
func {{ .Direction }}_{{ .MigrationId }}() error {
	return nil
}`

func renderGoMigrationToFile(filePath string, state migrationInfo) error {
	migrationFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer migrationFile.Close()

	tmpl, err := template.New("go-migration-template").Parse(goMigrationTemplate)
	if err != nil {
		return err
	}

	err = tmpl.Execute(migrationFile, state)
	if err != nil {
		return err
	}

	return nil
}

func (c *GenerateCommand) GenerateGoMigration() error {
	currentTime := time.Now().Unix()
	fileNameFormat := "%d_%s.%s.go"

	upMigrationFileName := fmt.Sprintf(fileNameFormat, currentTime, c.MigrationName, "up")
	downMigrationFileName := fmt.Sprintf(fileNameFormat, currentTime, c.MigrationName, "down")

	err := renderGoMigrationToFile(path.Join(c.MigrationDirectory, upMigrationFileName), migrationInfo{
		MigrationId: currentTime,
		Direction:   "Up",
	})
	if err != nil {
		return err
	}

	err = renderGoMigrationToFile(path.Join(c.MigrationDirectory, downMigrationFileName), migrationInfo{
		MigrationId: currentTime,
		Direction:   "Down",
	})
	if err != nil {
		return err
	}

	return nil
}
