package algorithm

import (
	"database/sql"
	"fmt"
	"log"

	sq "github.com/Masterminds/squirrel"
)

type VersionCandidate struct {
	VersionID  int
	BuildID    int
	JobID      int
	CheckOrder int
}

func (candidate VersionCandidate) String() string {
	return fmt.Sprintf("{v%d, j%db%d}", candidate.VersionID, candidate.JobID, candidate.BuildID)
}

type VersionCandidates struct {
	runner sq.Runner

	versionsQuery sq.SelectBuilder
	buildIDs      map[int]BuildSet

	pinned bool

	everyLatest  int64
	everyForward bool
}

func (candidates VersionCandidates) Pinned() bool {
	return candidates.pinned
}

func (candidates VersionCandidates) BuildIDs(jobID int) BuildSet {
	builds, found := candidates.buildIDs[jobID]
	if !found {
		builds = BuildSet{}
	}

	return builds
}

func (candidates VersionCandidates) PruneVersionsOfOtherBuildIDs(jobID int, buildIDs BuildSet) VersionCandidates {
	var ids []int
	for id := range buildIDs {
		ids = append(ids, id)
	}

	newCandidates := candidates
	if candidates.buildIDs != nil {
		_, hasJob := candidates.buildIDs[jobID]
		if hasJob {
			newCandidates.versionsQuery = newCandidates.versionsQuery.Where(sq.Eq{fmt.Sprintf("o%d.build_id", jobID): ids})
		}
	} //else {
	// 	newCandidates.versionsQuery = newCandidates.versionsQuery.Where(sq.Or{
	// 		sq.NotEq{"b.job_id": jobID},
	// 		sq.Eq{"o.build_id": ids},
	// 	})
	// }
	return newCandidates
}

func (candidates VersionCandidates) ConsecutiveVersions(jobID int, resourceID int) (VersionCandidates, error) {
	var latestCheckOrder sql.NullInt64
	err := candidates.runner.QueryRow(`
	  SELECT COALESCE(MAX(v.check_order))
		FROM build_resource_config_version_inputs i
		JOIN builds b ON b.id = i.build_id AND b.job_id = $1
		JOIN resources r ON r.id = $2 AND i.resource_id = r.id
		JOIN resource_config_versions v ON v.resource_config_scope_id = r.resource_config_scope_id AND v.version_md5 = i.version_md5
	`, jobID, resourceID).Scan(&latestCheckOrder)
	if err != nil {
		return VersionCandidates{}, err
	}

	if !latestCheckOrder.Valid {
		return candidates, nil
	}

	newCandidates := candidates
	newCandidates.everyLatest = latestCheckOrder.Int64
	newCandidates.everyForward = true
	return newCandidates, nil
}

func (candidates VersionCandidates) ForVersion(versionID int) VersionCandidates {
	newCandidates := candidates
	newCandidates.versionsQuery = newCandidates.versionsQuery.Where(sq.Eq{
		"v.id": versionID,
	})
	newCandidates.pinned = true
	return newCandidates
}

const batchSize = 100

func (candidates VersionCandidates) VersionIDs() *VersionsIter {
	return &VersionsIter{
		query: candidates.versionsQuery,

		everyLatest:  candidates.everyLatest,
		everyForward: candidates.everyForward,
	}
}

type VersionsIter struct {
	query sq.SelectBuilder

	ids    []int
	offset int

	lastCheckOrder int
	exhausted      bool

	everyLatest  int64
	everyForward bool
}

func (iter *VersionsIter) Next() (int, bool, error) {
	if iter.offset >= len(iter.ids) {
		ok, err := iter.hydrate()
		if err != nil {
			return 0, false, err
		}

		if !ok {
			return 0, false, nil
		}
	}

	v := iter.ids[iter.offset]

	iter.offset++

	return v, true, nil
}

func (iter *VersionsIter) Peek() (int, bool, error) {
	if iter.offset >= len(iter.ids) {
		ok, err := iter.hydrate()
		if err != nil {
			return 0, false, err
		}

		if !ok {
			return 0, false, nil
		}
	}

	return iter.ids[iter.offset], true, nil
}

func (iter *VersionsIter) hydrate() (bool, error) {
	if iter.exhausted {
		return false, nil
	}

	query := iter.query.Limit(batchSize)

	if iter.lastCheckOrder != 0 {
		if iter.everyForward {
			query = query.Where(sq.Gt{"v.check_order": iter.lastCheckOrder})
		} else {
			query = query.Where(sq.Lt{"v.check_order": iter.lastCheckOrder})
		}
	}

	if iter.everyLatest == 0 {
		query = query.OrderBy("check_order DESC")
	} else {
		if iter.everyForward {
			query = query.Where(sq.Gt{"v.check_order": iter.everyLatest}).
				OrderBy("check_order ASC")
		} else {
			query = query.Where(sq.LtOrEq{"v.check_order": iter.everyLatest}).
				OrderBy("check_order DESC")
		}
	}

	log.Println(query.ToSql())

	rows, err := query.Query()
	if err != nil {
		return false, err
	}

	var newIDs []int
	for rows.Next() {
		var id, checkOrder int
		err := rows.Scan(&id, &checkOrder)
		if err != nil {
			return false, err
		}

		iter.lastCheckOrder = checkOrder

		newIDs = append(newIDs, id)
	}

	if len(newIDs) < batchSize {
		if iter.everyLatest != 0 && iter.everyForward {
			iter.lastCheckOrder = 0
			iter.everyForward = false

			if len(newIDs) == 0 {
				// nothing on first call; try going backwards immediately
				return iter.hydrate()
			}
		}

		iter.exhausted = true
	}

	iter.ids = newIDs
	iter.offset = 0

	return len(iter.ids) != 0, nil
}
