package db

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc/db/lock"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . ContainerRepository

type ContainerRepository interface {
	FindOrphanedContainers() ([]CreatingContainer, []CreatedContainer, []DestroyingContainer, error)
	DestroyFailedContainers() (int, error)
	FindDestroyingContainers(workerName string) ([]string, error)
	RemoveDestroyingContainers(workerName string, currentHandles []string) (int, error)
	UpdateContainersMissingSince(workerName string, handles []string) error
	RemoveMissingContainers(time.Duration) (int, error)
	DestroyUnknownContainers(workerName string, reportedHandles []string) (int, error)
	VisibleContainers([]string) ([]Container, error)
	AllContainers() ([]Container, error)
}

type containerRepository struct {
	conn        Conn
	lockFactory lock.LockFactory
}

var containerColumns []string = append([]string{"id", "handle", "worker_name", "hijacked", "discontinued", "state"}, containerMetadataColumns...)

func NewContainerRepository(conn Conn, lockFactory lock.LockFactory) ContainerRepository {
	return &containerRepository{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func diff(a, b []string) (diff []string) {
	m := make(map[string]bool)

	for _, item := range b {
		m[item] = true
	}

	for _, item := range a {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}

	return
}

func (repository *containerRepository) queryContainerHandles(tx Tx, cond sq.Eq) ([]string, error) {
	query, args, err := psql.Select("handle").From("containers").Where(cond).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	handles := []string{}

	for rows.Next() {
		var handle = "handle"
		columns := []interface{}{&handle}

		err = rows.Scan(columns...)
		if err != nil {
			return nil, err
		}
		handles = append(handles, handle)
	}

	return handles, nil
}

func (repository *containerRepository) UpdateContainersMissingSince(workerName string, reportedHandles []string) error {
	// clear out missing_since for reported containers
	query, args, err := psql.Update("containers").
		Set("missing_since", nil).
		Where(
			sq.And{
				sq.NotEq{
					"missing_since": nil,
				},
				sq.Eq{
					"handle": reportedHandles,
				},
			},
		).ToSql()
	if err != nil {
		return err
	}

	tx, err := repository.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	rows, err := tx.Query(query, args...)
	if err != nil {
		return err
	}

	Close(rows)

	dbHandles, err := repository.queryContainerHandles(tx, sq.Eq{
		"worker_name":   workerName,
		"missing_since": nil,
	})
	if err != nil {
		return err
	}

	handles := diff(dbHandles, reportedHandles)

	query, args, err = psql.Update("containers").
		Set("missing_since", sq.Expr("now()")).
		Where(sq.And{
			sq.Eq{"handle": handles},
			sq.NotEq{"state": atc.ContainerStateCreating},
		}).ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(query, args...)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (repository *containerRepository) FindDestroyingContainers(workerName string) ([]string, error) {
	tx, err := repository.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	destroyingContainers, err := repository.queryContainerHandles(
		tx,
		sq.Eq{
			"state":        atc.ContainerStateDestroying,
			"worker_name":  workerName,
			"discontinued": false,
		},
	)

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return destroyingContainers, err
}

func (repository *containerRepository) RemoveMissingContainers(gracePeriod time.Duration) (int, error) {
	result, err := psql.Delete("containers c USING workers w").
		Where(sq.Expr("c.worker_name = w.name")).
		Where(
			sq.And{
				sq.Expr(fmt.Sprintf("c.state='%s'", atc.ContainerStateCreated)),
				sq.Expr(fmt.Sprintf("w.state!='%s'", WorkerStateStalled)),
				sq.Expr(fmt.Sprintf("NOW() - missing_since > '%s'", fmt.Sprintf("%.0f seconds", gracePeriod.Seconds()))),
			},
		).RunWith(repository.conn).
		Exec()

	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

func (repository *containerRepository) RemoveDestroyingContainers(workerName string, handlesToIgnore []string) (int, error) {
	rows, err := psql.Delete("containers").
		Where(
			sq.And{
				sq.Eq{
					"worker_name": workerName,
				},
				sq.NotEq{
					"handle": handlesToIgnore,
				},
				sq.Eq{
					"state": atc.ContainerStateDestroying,
				},
			},
		).RunWith(repository.conn).
		Exec()

	if err != nil {
		return 0, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

func (repository *containerRepository) FindOrphanedContainers() ([]CreatingContainer, []CreatedContainer, []DestroyingContainer, error) {
	query, args, err := selectContainers(nil, "c").
		LeftJoin("builds b ON b.id = c.build_id").
		LeftJoin("containers icc ON icc.id = c.image_check_container_id").
		LeftJoin("containers igc ON igc.id = c.image_get_container_id").
		Where(sq.Or{
			sq.Eq{
				"c.build_id":                         nil,
				"c.image_check_container_id":         nil,
				"c.image_get_container_id":           nil,
				"c.resource_config_check_session_id": nil,
			},
			sq.And{
				sq.NotEq{"c.build_id": nil},
				sq.Eq{"b.interceptible": false},
			},
			sq.And{
				sq.NotEq{"c.image_check_container_id": nil},
				sq.NotEq{"icc.state": atc.ContainerStateCreating},
			},
			sq.And{
				sq.NotEq{"c.image_get_container_id": nil},
				sq.NotEq{"igc.state": atc.ContainerStateCreating},
			},
		}).
		ToSql()
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, nil, nil, err
	}

	defer Close(rows)

	creatingContainers := []CreatingContainer{}
	createdContainers := []CreatedContainer{}
	destroyingContainers := []DestroyingContainer{}

	var (
		creatingContainer   CreatingContainer
		createdContainer    CreatedContainer
		destroyingContainer DestroyingContainer
	)

	for rows.Next() {
		creatingContainer, createdContainer, destroyingContainer, _, err = scanContainer(rows, false, repository.conn)
		if err != nil {
			return nil, nil, nil, err
		}

		if creatingContainer != nil {
			creatingContainers = append(creatingContainers, creatingContainer)
		}

		if createdContainer != nil {
			createdContainers = append(createdContainers, createdContainer)
		}

		if destroyingContainer != nil {
			destroyingContainers = append(destroyingContainers, destroyingContainer)
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, nil, nil, err
	}

	return creatingContainers, createdContainers, destroyingContainers, nil
}

func selectContainers(extraColumns []string, asOptional ...string) sq.SelectBuilder {
	localContainerColumns := make([]string, len(containerColumns))
	copy(localContainerColumns, containerColumns)

	table := "containers"
	if len(asOptional) > 0 {
		as := asOptional[0]
		for i, col := range localContainerColumns {
			localContainerColumns[i] = as + "." + col
		}

		table += " " + as
	}

	localContainerColumns = append(localContainerColumns, extraColumns...)
	return psql.Select(localContainerColumns...).From(table)
}

func selectContainersWithAdditionalColumns(asOptional ...string) sq.SelectBuilder {

	localContainerColumns := make([]string, len(containerColumns))

	table := "containers"
	as := "c"
	if len(asOptional) > 0 {
		as = asOptional[0]
	}

	for i, col := range containerColumns {
		localContainerColumns[i] = as + "." + col
	}

	table += " " + as
	localContainerColumns = append(localContainerColumns, "COALESCE(t.name, '') as team_name")
	return psql.Select(localContainerColumns...).From(table)
}

func scanContainer(row sq.RowScanner, scanTeamName bool, conn Conn) (CreatingContainer, CreatedContainer, DestroyingContainer, FailedContainer, error) {
	var (
		id             int
		handle         string
		workerName     string
		teamName       string
		isDiscontinued bool
		isHijacked     bool
		state          string

		metadata ContainerMetadata
	)

	columns := []interface{}{&id, &handle, &workerName, &isHijacked, &isDiscontinued, &state}
	columns = append(columns, metadata.ScanTargets()...)
	if scanTeamName {
		columns = append(columns, &teamName)
	}

	err := row.Scan(columns...)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	switch state {
	case atc.ContainerStateCreating:
		return newCreatingContainer(
			id,
			handle,
			workerName,
			teamName,
			metadata,
			conn,
		), nil, nil, nil, nil
	case atc.ContainerStateCreated:
		return nil, newCreatedContainer(
			id,
			handle,
			workerName,
			teamName,
			metadata,
			isHijacked,
			conn,
		), nil, nil, nil
	case atc.ContainerStateDestroying:
		return nil, nil, newDestroyingContainer(
			id,
			handle,
			workerName,
			teamName,
			metadata,
			isDiscontinued,
			conn,
		), nil, nil
	case atc.ContainerStateFailed:
		return nil, nil, nil, newFailedContainer(
			id,
			handle,
			workerName,
			teamName,
			metadata,
			conn,
		), nil
	}

	return nil, nil, nil, nil, nil
}

func (repository *containerRepository) DestroyFailedContainers() (int, error) {
	result, err := psql.Update("containers").
		Set("state", atc.ContainerStateDestroying).
		Where(sq.Eq{"state": string(atc.ContainerStateFailed)}).
		RunWith(repository.conn).
		Exec()

	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

func (repository *containerRepository) DestroyUnknownContainers(workerName string, reportedHandles []string) (int, error) {
	tx, err := repository.conn.Begin()
	if err != nil {
		return 0, err
	}

	defer Rollback(tx)

	dbHandles, err := repository.queryContainerHandles(tx, sq.Eq{
		"worker_name": workerName,
	})
	if err != nil {
		return 0, err
	}

	unknownHandles := diff(reportedHandles, dbHandles)

	if len(unknownHandles) == 0 {
		return 0, nil
	}

	insertBuilder := psql.Insert("containers").Columns(
		"handle",
		"worker_name",
		"state",
	)
	for _, unknownHandle := range unknownHandles {
		insertBuilder = insertBuilder.Values(
			unknownHandle,
			workerName,
			atc.ContainerStateDestroying,
		)
	}
	_, err = insertBuilder.RunWith(tx).Exec()
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(unknownHandles), nil
}

func (repository *containerRepository) AllContainers() ([]Container, error) {
	return getContainersByTeamIds([]int{}, true, repository.conn)
}

func getContainersByTeamIds(teamIds []int, includeTeamName bool, conn Conn) ([]Container, error) {
	//selects the resource check containers that are associated to a pipeline that is associated to this team
	//and is running on either the team's worker or a general worker.
	var teamColumn []string
	if includeTeamName {
		teamColumn = []string{"COALESCE(t.name, '') as team_name"}
	}

	rowsBuilder := selectContainers(teamColumn, "c").
		Join("workers w ON c.worker_name = w.name").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Join("resources r ON r.resource_config_id = rccs.resource_config_id").
		Join("pipelines p ON p.id = r.pipeline_id")

	if includeTeamName {
		rowsBuilder = rowsBuilder.LeftJoin("teams t ON t.id = w.team_id")
	}

	if len(teamIds) > 0 {
		rowsBuilder = rowsBuilder.Where(sq.Eq{
			"p.team_id": teamIds,
		}).
			Where(sq.Or{
				sq.Eq{
					"w.team_id": teamIds,
				}, sq.Eq{
					"w.team_id": nil,
				},
			})
	}

	rows, err := rowsBuilder.Distinct().RunWith(conn).Query()
	if err != nil {
		return nil, err
	}

	var containers []Container
	containers, err = scanContainers(rows, conn, includeTeamName, containers)
	if err != nil {
		return nil, err
	}

	//selects the resource_types check containers that are associated to a pipeline that is associated to this team
	//and is running on either the team's worker or a general worker.
	rowsBuilder = selectContainers(teamColumn, "c").
		Join("workers w ON c.worker_name = w.name").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Join("resource_types rt ON rt.resource_config_id = rccs.resource_config_id").
		Join("pipelines p ON p.id = rt.pipeline_id")

	if includeTeamName {
		rowsBuilder = rowsBuilder.LeftJoin("teams t ON t.id = w.team_id")
	}

	if len(teamIds) > 0 {
		rowsBuilder = rowsBuilder.Where(sq.Eq{
			"p.team_id": teamIds,
		}).
			Where(sq.Or{
				sq.Eq{
					"w.team_id": teamIds,
				}, sq.Eq{
					"w.team_id": nil,
				},
			})
	}

	rows, err = rowsBuilder.Distinct().RunWith(conn).Query()
	if err != nil {
		return nil, err
	}

	containers, err = scanContainers(rows, conn, includeTeamName, containers)
	if err != nil {
		return nil, err
	}

	//selecting the step containers that are directly associated to the team
	rowsBuilder = selectContainers(teamColumn, "c")

	if includeTeamName {
		rowsBuilder = rowsBuilder.LeftJoin("teams t ON t.id = c.team_id")
	}

	if len(teamIds) > 0 {
		rowsBuilder = rowsBuilder.Where(sq.Eq{
			"c.team_id": teamIds,
		})
	} else {
		rowsBuilder = rowsBuilder.Where(sq.NotEq{
			"c.team_id": nil,
		})
	}

	rows, err = rowsBuilder.
		RunWith(conn).
		Query()
	if err != nil {
		return nil, err
	}

	containers, err = scanContainers(rows, conn, includeTeamName, containers)
	if err != nil {
		return nil, err
	}

	return containers, nil
}

func (repository *containerRepository) VisibleContainers(teamNames []string) ([]Container, error) {
	//query all the team ids
	var teamIds []int
	teamRows, teamErr := psql.Select("id").
		From("teams").
		Where(sq.Eq{"name": teamNames}).
		RunWith(repository.conn).
		Query()
	if teamErr != nil {
		return nil, teamErr
	}
	for teamRows.Next() {
		var teamId int
		err := teamRows.Scan(&teamId)
		if err != nil {
			return nil, err
		}
		teamIds = append(teamIds, teamId)
	}

	return getContainersByTeamIds(teamIds, true, repository.conn)
}
