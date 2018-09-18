package migration

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var noTxPrefix = regexp.MustCompile("^\\s*--\\s+(NO_TRANSACTION)")
var migrationDirection = regexp.MustCompile("\\.(up|down)\\.")
var goMigrationFuncName = regexp.MustCompile("(Up|Down)_[0-9]*")

var ErrCouldNotParseDirection = errors.New("could not parse direction for migration")

type Parser struct {
	bindata Bindata
}

func NewParser(bindata Bindata) *Parser {
	return &Parser{
		bindata: bindata,
	}
}

func (p *Parser) ParseMigrationFilename(fileName string) (migration, error) {
	var (
		migration migration
		err       error
	)

	migration.Direction, err = determineDirection(fileName)
	if err != nil {
		return migration, err
	}

	migration.Version, err = schemaVersion(fileName)
	if err != nil {
		return migration, err
	}

	return migration, nil
}

func (p *Parser) ParseFileToMigration(migrationName string) (migration, error) {
	var migrationContents string

	migration, err := p.ParseMigrationFilename(migrationName)
	if err != nil {
		return migration, err
	}

	migrationBytes, err := p.bindata.Asset(migrationName)
	if err != nil {
		return migration, err
	}

	migrationContents = string(migrationBytes)
	migration.Strategy = determineMigrationStrategy(migrationName, migrationContents)

	switch migration.Strategy {
	case GoMigration:
		migration.Name = goMigrationFuncName.FindString(migrationContents)
	case SQLNoTransaction:
		migration.Statements = []string{migrationContents}
		migration.Name = migrationName
	case SQLTransaction:
		migration.Statements = splitStatements(migrationContents)
		migration.Name = migrationName
	}

	return migration, nil
}

func schemaVersion(assetName string) (int, error) {
	regex := regexp.MustCompile("(\\d+)")
	match := regex.FindStringSubmatch(assetName)
	return strconv.Atoi(match[1])
}

func determineDirection(migrationName string) (string, error) {
	matches := migrationDirection.FindStringSubmatch(migrationName)
	if len(matches) < 2 {
		return "", ErrCouldNotParseDirection
	}

	return matches[1], nil
}

func determineMigrationStrategy(migrationName string, migrationContents string) Strategy {
	if strings.HasSuffix(migrationName, ".go") {
		return GoMigration
	} else {
		if noTxPrefix.MatchString(migrationContents) {
			return SQLNoTransaction
		}
	}
	return SQLTransaction
}

func splitStatements(migrationContents string) []string {
	var (
		fileStatements      []string
		migrationStatements []string
	)
	fileStatements = append(fileStatements, strings.Split(migrationContents, ";")...)
	// last string is empty
	if strings.TrimSpace(fileStatements[len(fileStatements)-1]) == "" {
		fileStatements = fileStatements[:len(fileStatements)-1]
	}

	var isSqlStatement bool = false
	var sqlStatement string
	for _, statement := range fileStatements {
		statement = strings.TrimSpace(statement)

		if statement == "BEGIN" || statement == "COMMIT" {
			continue
		}
		if strings.Contains(statement, "BEGIN") {
			isSqlStatement = true
			sqlStatement = statement + ";"
		} else {

			if isSqlStatement {
				sqlStatement = strings.Join([]string{sqlStatement, statement, ";"}, "")
				if strings.HasPrefix(statement, "$$") {
					migrationStatements = append(migrationStatements, sqlStatement)
					isSqlStatement = false
				}
			} else {
				migrationStatements = append(migrationStatements, statement)
			}
		}
	}
	return migrationStatements
}
