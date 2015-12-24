package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
)

type SQLDB struct {
	logger lager.Logger

	conn Conn
	bus  *notificationsBus
}

func NewSQL(
	logger lager.Logger,
	sqldbConnection Conn,
	bus *notificationsBus,
) *SQLDB {
	return &SQLDB{
		logger: logger,

		conn: sqldbConnection,
		bus:  bus,
	}
}
func (db *SQLDB) CreateDefaultTeamIfNotExists() error {
	_, err := db.conn.Exec(`
	INSERT INTO teams (
    name, admin
	)
	SELECT $1, true
	WHERE NOT EXISTS (
		SELECT id FROM teams WHERE name = $1
	)
	`, atc.DefaultTeamName)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		UPDATE teams
		SET admin = true
		WHERE name = $1
	`, atc.DefaultTeamName)
	return err
}

func (db *SQLDB) SaveTeam(data Team) (SavedTeam, error) {
	jsonEncodedBasicAuth, err := db.jsonEncodeTeamBasicAuth(data)
	if err != nil {
		return SavedTeam{}, err
	}
	jsonEncodedGitHubAuth, err := db.jsonEncodeTeamGitHubAuth(data)
	if err != nil {
		return SavedTeam{}, err
	}

	return db.queryTeam(fmt.Sprintf(`
	INSERT INTO teams (
    name, basic_auth, github_auth
	) VALUES (
		'%s', '%s', '%s'
	)
	RETURNING id, name, admin, basic_auth, github_auth
	`, data.Name, jsonEncodedBasicAuth, jsonEncodedGitHubAuth,
	))
}

func (db *SQLDB) queryTeam(query string) (SavedTeam, error) {
	var basicAuth, gitHubAuth sql.NullString
	var savedTeam SavedTeam

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedTeam{}, err
	}
	defer tx.Rollback()

	err = tx.QueryRow(query).Scan(
		&savedTeam.ID,
		&savedTeam.Name,
		&savedTeam.Admin,
		&basicAuth,
		&gitHubAuth,
	)
	if err != nil {
		return savedTeam, err
	}
	err = tx.Commit()
	if err != nil {
		return savedTeam, err
	}

	if basicAuth.Valid {
		err = json.Unmarshal([]byte(basicAuth.String), &savedTeam.BasicAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	if gitHubAuth.Valid {
		err = json.Unmarshal([]byte(gitHubAuth.String), &savedTeam.GitHubAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	return savedTeam, nil
}

func (db *SQLDB) GetTeamByName(teamName string) (SavedTeam, bool, error) {
	query := fmt.Sprintf(`
		SELECT id, name, admin, basic_auth, github_auth
		FROM teams
		WHERE name = '%s'
	`, teamName,
	)
	savedTeam, err := db.queryTeam(query)
	if err != nil {
		if err == sql.ErrNoRows {
			return savedTeam, false, nil
		}

		return savedTeam, false, err
	}

	return savedTeam, true, err
}

func (db *SQLDB) jsonEncodeTeamGitHubAuth(team Team) (string, error) {
	if team.ClientID == "" || team.ClientSecret == "" {
		team.GitHubAuth = GitHubAuth{}
	}

	json, err := json.Marshal(team.GitHubAuth)
	return string(json), err
}

func (db *SQLDB) UpdateTeamGitHubAuth(team Team) (SavedTeam, error) {
	gitHubAuth, err := db.jsonEncodeTeamGitHubAuth(team)
	if err != nil {
		return SavedTeam{}, err
	}

	query := fmt.Sprintf(`
		UPDATE teams
		SET github_auth = '%s'
		WHERE name = '%s'
		RETURNING id, name, admin, basic_auth, github_auth
	`, gitHubAuth, team.Name,
	)
	return db.queryTeam(query)
}

func (db *SQLDB) jsonEncodeTeamBasicAuth(team Team) (string, error) {
	if team.BasicAuthUsername == "" || team.BasicAuthPassword == "" {
		team.BasicAuth = BasicAuth{}
	} else {
		encryptedPw, err := bcrypt.GenerateFromPassword([]byte(team.BasicAuthPassword), 11)
		if err != nil {
			return "", err
		}
		team.BasicAuthPassword = string(encryptedPw)
	}

	json, err := json.Marshal(team.BasicAuth)
	return string(json), err
}

func (db *SQLDB) UpdateTeamBasicAuth(team Team) (SavedTeam, error) {
	basicAuth, err := db.jsonEncodeTeamBasicAuth(team)
	if err != nil {
		return SavedTeam{}, err
	}

	query := fmt.Sprintf(`
		UPDATE teams
		SET basic_auth = '%s'
		WHERE name = '%s'
		RETURNING id, name, admin, basic_auth, github_auth
	`, basicAuth, team.Name)
	return db.queryTeam(query)
}

func (db *SQLDB) DeleteTeamByName(teamName string) error {
	_, err := db.conn.Exec(`
    DELETE FROM teams
		WHERE name = $1
	`, teamName)
	return err
}

func (db *SQLDB) CreatePipe(pipeGUID string, url string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO pipes(id, url)
		VALUES ($1, $2)
	`, pipeGUID, url)

	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) GetPipe(pipeGUID string) (Pipe, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Pipe{}, err
	}

	defer tx.Rollback()

	var pipe Pipe

	err = tx.QueryRow(`
		SELECT id, coalesce(url, '') AS url
		FROM pipes
		WHERE id = $1
	`, pipeGUID).Scan(&pipe.ID, &pipe.URL)

	if err != nil {
		return Pipe{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Pipe{}, err
	}

	return pipe, nil
}

func (db *SQLDB) LeaseBuildTracking(buildID int, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"build_id": buildID,
		}),
		attemptSignFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_tracked = now()
				WHERE id = $1
					AND now() - last_tracked > ($2 || ' SECONDS')::INTERVAL
			`, buildID, interval.Seconds())
		},
		heartbeatFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_tracked = now()
				WHERE id = $1
			`, buildID)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (db *SQLDB) LeaseBuildScheduling(buildID int, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"build_id": buildID,
		}),
		attemptSignFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
					AND now() - last_scheduled > ($2 || ' SECONDS')::INTERVAL
			`, buildID, interval.Seconds())
		},
		heartbeatFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
			`, buildID)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (db *SQLDB) LeaseCacheInvalidation(interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"CacheInvalidator": "Scottsboro",
		}),
		attemptSignFunc: func(tx *sql.Tx) (sql.Result, error) {
			_, err := tx.Exec(`
				INSERT INTO cache_invalidator (last_invalidated)
				SELECT 'epoch'
				WHERE NOT EXISTS (SELECT * FROM cache_invalidator)`)
			if err != nil {
				return nil, err
			}
			return tx.Exec(`
				UPDATE cache_invalidator
				SET last_invalidated = now()
				WHERE now() - last_invalidated > ($1 || ' SECONDS')::INTERVAL
			`, interval.Seconds())
		},
		heartbeatFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE cache_invalidator
				SET last_invalidated = now()
			`)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

type nonOneRowAffectedError struct {
	RowsAffected int64
}

func (err nonOneRowAffectedError) Error() string {
	return fmt.Sprintf("expected 1 row to be updated; got %d", err.RowsAffected)
}

type scannable interface {
	Scan(destinations ...interface{}) error
}

func newConditionNotifier(bus *notificationsBus, channel string, cond func() (bool, error)) (Notifier, error) {
	notified, err := bus.Listen(channel)
	if err != nil {
		return nil, err
	}

	notifier := &conditionNotifier{
		cond:    cond,
		bus:     bus,
		channel: channel,

		notified: notified,
		notify:   make(chan struct{}, 1),

		stop: make(chan struct{}),
	}

	go notifier.watch()

	return notifier, nil
}

type conditionNotifier struct {
	cond func() (bool, error)

	bus     *notificationsBus
	channel string

	notified chan bool
	notify   chan struct{}

	stop chan struct{}
}

func (notifier *conditionNotifier) Notify() <-chan struct{} {
	return notifier.notify
}

func (notifier *conditionNotifier) Close() error {
	close(notifier.stop)
	return notifier.bus.Unlisten(notifier.channel, notifier.notified)
}

func (notifier *conditionNotifier) watch() {
	for {
		c, err := notifier.cond()
		if err != nil {
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-notifier.stop:
				return
			}
		}

		if c {
			notifier.sendNotification()
		}

	dance:
		for {
			select {
			case <-notifier.stop:
				return
			case ok := <-notifier.notified:
				if ok {
					notifier.sendNotification()
				} else {
					break dance
				}
			}
		}
	}
}

func (notifier *conditionNotifier) sendNotification() {
	select {
	case notifier.notify <- struct{}{}:
	default:
	}
}
