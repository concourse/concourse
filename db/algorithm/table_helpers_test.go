package algorithm_test

import (
	"fmt"

	. "github.com/concourse/atc/db/algorithm"
	. "github.com/onsi/gomega"
)

type DB []DBRow

type DBRow struct {
	Job      string
	BuildID  int
	Resource string
	Version  string
}

type Example struct {
	FIt    string
	It     string
	DB     DB
	Inputs Inputs
	Result Result
}

type Inputs []Input

type Input struct {
	Name     string
	Resource string
	Passed   []string
}

type Result map[string]string

type StringMapping map[string]int

func (mapping StringMapping) ID(str string) int {
	id, found := mapping[str]
	if !found {
		id = len(mapping) + 1
		mapping[str] = id
	}

	return id
}

func (mapping StringMapping) Name(id int) string {
	for mappingName, mappingID := range mapping {
		if id == mappingID {
			return mappingName
		}
	}

	panic(fmt.Sprintf("no name found for %d", id))
}

func runExample(example Example) {
	jobIDs := StringMapping{}
	resourceIDs := StringMapping{}
	versionIDs := StringMapping{}

	inputConfigs := make(InputConfigs, len(example.Inputs))
	for i, input := range example.Inputs {
		passed := JobSet{}
		for _, jobName := range input.Passed {
			passed[jobIDs.ID(jobName)] = struct{}{}
		}

		inputConfigs[i] = InputConfig{
			Name:       input.Name,
			Passed:     passed,
			ResourceID: resourceIDs.ID(input.Resource),
		}
	}

	db := &VersionsDB{}

	for _, row := range example.DB {
		version := ResourceVersion{
			VersionID:  versionIDs.ID(row.Version),
			ResourceID: resourceIDs.ID(row.Resource),
		}

		if row.Job != "" {
			db.BuildOutputs = append(db.BuildOutputs, BuildOutput{
				ResourceVersion: version,
				BuildID:         row.BuildID,
				JobID:           jobIDs.ID(row.Job),
			})
		} else {
			db.ResourceVersions = append(db.ResourceVersions, version)
		}
	}

	result, ok := inputConfigs.Resolve(db)
	Expect(ok).To(BeTrue())

	prettyResult := Result{}
	for name, versionID := range result {
		prettyResult[name] = versionIDs.Name(versionID)
	}

	Expect(prettyResult).To(Equal(example.Result))
}
