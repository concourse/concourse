package migration

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var migrationDirection = regexp.MustCompile(`\.(up|down)\.`)
var goMigrationFuncName = regexp.MustCompile(`(Up|Down)_[0-9]*`)

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
	case SQLMigration:
		migration.Name = migrationName
		migration.Statements = migrationContents
	}

	return migration, nil
}

func schemaVersion(assetName string) (int, error) {
	regex := regexp.MustCompile(`(\d+)`)
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
		return SQLMigration
	}
}
