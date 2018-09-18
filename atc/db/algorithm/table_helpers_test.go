package algorithm_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"

	"github.com/concourse/atc/db/algorithm"
	. "github.com/onsi/gomega"
)

type DB struct {
	BuildInputs  []DBRow
	BuildOutputs []DBRow
	Resources    []DBRow
}

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
	Version  Version
}

type Version struct {
	Every  bool
	Latest bool
	Pinned string
}

type Result struct {
	OK     bool
	Values map[string]string
}

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

const CurrentJobName = "current"

func (example Example) Run() {
	db := &algorithm.VersionsDB{}

	jobIDs := StringMapping{}
	resourceIDs := StringMapping{}
	versionIDs := StringMapping{}

	if example.LoadDB != "" {
		dbFile, err := os.Open(example.LoadDB)
		Expect(err).ToNot(HaveOccurred())

		gr, err := gzip.NewReader(dbFile)
		Expect(err).ToNot(HaveOccurred())

		err = json.NewDecoder(gr).Decode(db)
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
		for _, row := range example.DB.Resources {
			version := algorithm.ResourceVersion{
				VersionID:  versionIDs.ID(row.Version),
				ResourceID: resourceIDs.ID(row.Resource),
				CheckOrder: row.CheckOrder,
			}
			db.ResourceVersions = append(db.ResourceVersions, version)
		}
		for _, row := range example.DB.BuildInputs {
			version := algorithm.ResourceVersion{
				VersionID:  versionIDs.ID(row.Version),
				ResourceID: resourceIDs.ID(row.Resource),
				CheckOrder: row.CheckOrder,
			}
			db.BuildInputs = append(db.BuildInputs, algorithm.BuildInput{
				ResourceVersion: version,
				BuildID:         row.BuildID,
				JobID:           jobIDs.ID(row.Job),
			})
		}
		for _, row := range example.DB.BuildOutputs {
			version := algorithm.ResourceVersion{
				VersionID:  versionIDs.ID(row.Version),
				ResourceID: resourceIDs.ID(row.Resource),
				CheckOrder: row.CheckOrder,
			}
			db.BuildOutputs = append(db.BuildOutputs, algorithm.BuildOutput{
				ResourceVersion: version,
				BuildID:         row.BuildID,
				JobID:           jobIDs.ID(row.Job),
			})
		}
	}

	inputConfigs := make(algorithm.InputConfigs, len(example.Inputs))
	for i, input := range example.Inputs {
		passed := algorithm.JobSet{}
		for _, jobName := range input.Passed {
			passed[jobIDs.ID(jobName)] = struct{}{}
		}

		var versionID int
		if input.Version.Pinned != "" {
			versionID = versionIDs.ID(input.Version.Pinned)
		}

		inputConfigs[i] = algorithm.InputConfig{
			Name:            input.Name,
			Passed:          passed,
			ResourceID:      resourceIDs.ID(input.Resource),
			UseEveryVersion: input.Version.Every,
			PinnedVersionID: versionID,
			JobID:           jobIDs.ID(CurrentJobName),
		}
	}

	resolved, ok := inputConfigs.Resolve(db)

	prettyValues := map[string]string{}
	for name, inputVersion := range resolved {
		prettyValues[name] = versionIDs.Name(inputVersion.VersionID)
	}

	actualResult := Result{OK: ok, Values: prettyValues}

	Expect(actualResult).To(Equal(example.Result))
}
