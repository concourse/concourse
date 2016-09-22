package algorithm

import (
	"fmt"
	"os"
	"strings"

	influxdb "github.com/influxdata/influxdb/client/v2"
)

var influxdbClient influxdb.Client
var influxdbDatabase string

func init() {
	influxdbDatabase = os.Getenv("ALGORITHM_INFLUXDB_DATABASE")

	addr := os.Getenv("ALGORITHM_INFLUXDB_URL")
	if addr != "" {
		var err error
		influxdbClient, err = influxdb.NewHTTPClient(influxdb.HTTPConfig{
			Addr: addr,
		})
		if err != nil {
			panic(err)
		}
	}
}

type InputCandidates []InputVersionCandidates

type ResolvedInputs map[string]int

type InputVersionCandidates struct {
	Input                 string
	Passed                JobSet
	UseEveryVersion       bool
	PinnedVersionID       int
	ExistingBuildResolver *ExistingBuildResolver
	usingEveryVersion     *bool

	VersionCandidates
}

func (inputVersionCandidates InputVersionCandidates) IsNext(version int, versionIDs *VersionsIter) bool {
	if !inputVersionCandidates.UsingEveryVersion() {
		return true
	}

	if inputVersionCandidates.ExistingBuildResolver.ExistsForVersion(version) {
		return true
	}

	next, hasNext := versionIDs.Peek()
	return !hasNext ||
		inputVersionCandidates.ExistingBuildResolver.ExistsForVersion(next)
}

func (inputVersionCandidates InputVersionCandidates) UsingEveryVersion() bool {
	if inputVersionCandidates.usingEveryVersion == nil {
		usingEveryVersion := inputVersionCandidates.UseEveryVersion &&
			inputVersionCandidates.ExistingBuildResolver.Exists()
		inputVersionCandidates.usingEveryVersion = &usingEveryVersion
	}

	return *inputVersionCandidates.usingEveryVersion
}

func (candidates InputCandidates) String() string {
	lens := []string{}
	for _, vcs := range candidates {
		lens = append(lens, fmt.Sprintf("%s (%d versions)", vcs.Input, vcs.VersionCandidates.Len()))
	}

	return fmt.Sprintf("[%s]", strings.Join(lens, "; "))
}

func send(point *influxdb.Point, pointErr error) {
	if pointErr != nil {
		panic(pointErr)
	}

	batch, err := influxdb.NewBatchPoints(influxdb.BatchPointsConfig{
		Database: influxdbDatabase,
	})
	if err != nil {
		panic(err)
	}

	batch.AddPoint(point)

	err = influxdbClient.Write(batch)
	if err != nil {
		panic(err)
	}
}

func (candidates InputCandidates) Reduce(depth int, jobs JobSet) (ResolvedInputs, bool) {
	newInputCandidates := candidates.pruneToCommonBuilds(jobs)

	for i, inputVersionCandidates := range newInputCandidates {
		if influxdbClient != nil {
			send(influxdb.NewPoint("depth", map[string]string{
				"input": inputVersionCandidates.Input,
			}, map[string]interface{}{
				"depth": depth,
			}))
		}

		if inputVersionCandidates.Len() == 1 {
			if influxdbClient != nil {
				send(influxdb.NewPoint("already-reduced", map[string]string{
					"input": inputVersionCandidates.Input,
				}, map[string]interface{}{
					"depth": depth,
				}))
			}

			continue
		}

		if inputVersionCandidates.PinnedVersionID != 0 {
			if influxdbClient != nil {
				send(influxdb.NewPoint("pinned", map[string]string{
					"input": inputVersionCandidates.Input,
				}, map[string]interface{}{
					"depth":   depth,
					"version": inputVersionCandidates.PinnedVersionID,
				}))
			}

			newInputCandidates.Pin(i, inputVersionCandidates.PinnedVersionID)
			continue
		}

		versionIDs := inputVersionCandidates.VersionIDs()

		iteration := 0

		for {
			id, ok := versionIDs.Next()
			if !ok {
				if influxdbClient != nil {
					send(influxdb.NewPoint("exhausted", map[string]string{
						"input": inputVersionCandidates.Input,
					}, map[string]interface{}{
						"depth": depth,
					}))
				}

				return nil, false
			}

			iteration++

			newInputCandidates.Pin(i, id)

			if influxdbClient != nil {
				send(influxdb.NewPoint("trying", map[string]string{
					"input": inputVersionCandidates.Input,
				}, map[string]interface{}{
					"depth":     depth,
					"iteration": iteration,
				}))
			}

			mapping, ok := newInputCandidates.Reduce(depth+1, jobs)
			if ok && inputVersionCandidates.IsNext(id, versionIDs) {
				if influxdbClient != nil {
					send(influxdb.NewPoint("resolved", map[string]string{
						"input": inputVersionCandidates.Input,
					}, map[string]interface{}{
						"depth":     depth,
						"iteration": iteration,
					}))
				}

				return mapping, true
			}

			newInputCandidates.Unpin(i, inputVersionCandidates)

			if influxdbClient != nil {
				send(influxdb.NewPoint("failed", map[string]string{
					"input": inputVersionCandidates.Input,
				}, map[string]interface{}{
					"depth":     depth,
					"iteration": iteration,
				}))
			}
		}
	}

	resolved := ResolvedInputs{}

	for _, inputVersionCandidates := range newInputCandidates {
		vids := inputVersionCandidates.VersionIDs()

		vid, ok := vids.Next()
		if !ok {
			return nil, false
		}

		resolved[inputVersionCandidates.Input] = vid
	}

	return resolved, true
}

func (candidates InputCandidates) Pin(input int, version int) {
	limitedToVersion := candidates[input].ForVersion(version)

	inputCandidates := candidates[input]
	inputCandidates.VersionCandidates = limitedToVersion
	candidates[input] = inputCandidates
}

func (candidates InputCandidates) Unpin(input int, inputCandidates InputVersionCandidates) {
	candidates[input] = inputCandidates
}

func (candidates InputCandidates) pruneToCommonBuilds(jobs JobSet) InputCandidates {
	newCandidates := make(InputCandidates, len(candidates))
	copy(newCandidates, candidates)

	for jobID, _ := range jobs {
		commonBuildIDs := newCandidates.commonBuildIDs(jobID)

		for i, versionCandidates := range newCandidates {
			inputCandidates := versionCandidates
			inputCandidates.VersionCandidates = versionCandidates.PruneVersionsOfOtherBuildIDs(jobID, commonBuildIDs)
			newCandidates[i] = inputCandidates
		}
	}

	return newCandidates
}

func (candidates InputCandidates) commonBuildIDs(jobID int) BuildSet {
	firstTick := true

	commonBuildIDs := BuildSet{}

	for _, set := range candidates {
		setBuildIDs := set.BuildIDs(jobID)
		if len(setBuildIDs) == 0 {
			continue
		}

		if firstTick {
			for id := range setBuildIDs {
				commonBuildIDs[id] = struct{}{}
			}
		} else {
			for id := range commonBuildIDs {
				_, found := setBuildIDs[id]
				if !found {
					delete(commonBuildIDs, id)
				}
			}
		}

		firstTick = false
	}

	return commonBuildIDs
}
