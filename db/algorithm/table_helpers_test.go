package algorithm_test

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/concourse/atc/db/algorithm"
	. "github.com/onsi/gomega"
)

type DB []DBRow

type DBRow struct {
	Job        string
	BuildID    int
	Resource   string
	Version    string
	CheckOrder int
	VersionID  int
}

type Example struct {
	LoadDB string
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

func (example Example) Run() {
	db := &algorithm.VersionsDB{}

	jobIDs := StringMapping{}
	resourceIDs := StringMapping{}
	versionIDs := StringMapping{}

	if example.LoadDB != "" {
		dbFile, err := os.Open(example.LoadDB)
		Expect(err).ToNot(HaveOccurred())

		err = json.NewDecoder(dbFile).Decode(db)
		Expect(err).ToNot(HaveOccurred())

		for name, id := range db.JobIDs {
			jobIDs[name] = id
		}

		for name, id := range db.ResourceIDs {
			resourceIDs[name] = id
		}

		for _, v := range db.ResourceVersions {
			importedName := fmt.Sprintf("imported-r%dv%d", v.ResourceID, v.VersionID)
			versionIDs[importedName] = v.VersionID
		}
	} else {
		for _, row := range example.DB {
			version := algorithm.ResourceVersion{
				VersionID:  versionIDs.ID(row.Version),
				ResourceID: resourceIDs.ID(row.Resource),
				CheckOrder: row.CheckOrder,
			}

			if row.Job != "" {
				db.BuildOutputs = append(db.BuildOutputs, algorithm.BuildOutput{
					ResourceVersion: version,
					BuildID:         row.BuildID,
					JobID:           jobIDs.ID(row.Job),
				})
			} else {
				db.ResourceVersions = append(db.ResourceVersions, version)
			}
		}
	}

	inputConfigs := make(algorithm.InputConfigs, len(example.Inputs))
	for i, input := range example.Inputs {
		passed := algorithm.JobSet{}
		for _, jobName := range input.Passed {
			passed[jobIDs.ID(jobName)] = struct{}{}
		}

		inputConfigs[i] = algorithm.InputConfig{
			Name:       input.Name,
			Passed:     passed,
			ResourceID: resourceIDs.ID(input.Resource),
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
