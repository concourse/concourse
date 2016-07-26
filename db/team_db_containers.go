package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func (db *teamDB) FindContainersByDescriptors(id Container) ([]SavedContainer, error) {
	var err error
	// err := deleteExpired(db)
	// if err != nil {
	// 	return nil, err
	// }

	var whereCriteria []string
	var params []interface{}

	if id.ResourceName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("r.name = $%d", len(params)+1))
		params = append(params, id.ResourceName)
	}

	if id.StepName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("c.step_name = $%d", len(params)+1))
		params = append(params, id.StepName)
	}

	if id.JobName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("j.name = $%d", len(params)+1))
		params = append(params, id.JobName)
	}

	if id.PipelineName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("p.name = $%d", len(params)+1))
		params = append(params, id.PipelineName)
	}

	if id.BuildID != 0 {
		whereCriteria = append(whereCriteria, fmt.Sprintf("build_id = $%d", len(params)+1))
		params = append(params, id.BuildID)
	}

	if id.Type != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("type = $%d", len(params)+1))
		params = append(params, id.Type.String())
	}

	if id.WorkerName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("worker_name = $%d", len(params)+1))
		params = append(params, id.WorkerName)
	}

	if id.CheckType != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("check_type = $%d", len(params)+1))
		params = append(params, id.CheckType)
	}

	if id.BuildName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("b.name = $%d", len(params)+1))
		params = append(params, id.BuildName)
	}

	var checkSourceBlob []byte
	if id.CheckSource != nil {
		checkSourceBlob, err = json.Marshal(id.CheckSource)
		if err != nil {
			return nil, err
		}
		whereCriteria = append(whereCriteria, fmt.Sprintf("check_source = $%d", len(params)+1))
		params = append(params, checkSourceBlob)
	}

	if len(id.Attempts) > 0 {
		attemptsBlob, err := json.Marshal(id.Attempts)
		if err != nil {
			return nil, err
		}
		whereCriteria = append(whereCriteria, fmt.Sprintf("attempts = $%d", len(params)+1))
		params = append(params, attemptsBlob)
	}

	var rows *sql.Rows
	selectQuery := `
		SELECT ` + containerColumns + `
		FROM containers c ` + containerJoins

	if len(whereCriteria) > 0 {
		selectQuery += fmt.Sprintf(" WHERE %s", strings.Join(whereCriteria, " AND "))
	}

	rows, err = db.conn.Query(selectQuery, params...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	infos := []SavedContainer{}
	for rows.Next() {
		info, err := scanContainer(rows)

		if err != nil {
			return nil, err
		}

		infos = append(infos, info)
	}

	return infos, nil
}
