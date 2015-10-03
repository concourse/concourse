package db

import (
	"bytes"
	"database/sql"
	"time"

	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

func Explain(logger lager.Logger, conn Conn, clock clock.Clock, timeout time.Duration) Conn {
	return &explainConn{
		Conn: conn,

		clock:   clock,
		timeout: timeout,

		logger: logger.WithData(lager.Data{
			"timeout": timeout,
		}),
	}
}

type explainConn struct {
	Conn

	clock   clock.Clock
	timeout time.Duration

	logger lager.Logger
}

type result struct {
	rows *sql.Rows
	err  error
}

func (e *explainConn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	results := make(chan result)

	go func(results chan result) {
		rows, err := e.Conn.Query(query, args...)

		results <- result{
			rows: rows,
			err:  err,
		}
	}(results)

	timeout := e.clock.NewTimer(e.timeout)
	defer timeout.Stop()

	select {
	case res := <-results:
		return res.rows, res.err

	case <-timeout.C():
		e.explainQuery(query, args...)
	}

	res := <-results
	return res.rows, res.err
}

func (e *explainConn) QueryRow(query string, args ...interface{}) *sql.Row {
	results := make(chan *sql.Row)

	go func(results chan *sql.Row) {
		row := e.Conn.QueryRow(query, args...)

		results <- row
	}(results)

	timeout := e.clock.NewTimer(e.timeout)
	defer timeout.Stop()

	select {
	case row := <-results:
		return row

	case <-timeout.C():
		e.explainQuery(query, args...)
	}

	row := <-results
	return row
}

type execResult struct {
	result sql.Result
	err    error
}

func (e *explainConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	results := make(chan execResult)

	go func(results chan execResult) {
		result, err := e.Conn.Exec(query, args...)

		results <- execResult{
			result: result,
			err:    err,
		}
	}(results)

	timeout := e.clock.NewTimer(e.timeout)
	defer timeout.Stop()

	select {
	case res := <-results:
		return res.result, res.err

	case <-timeout.C():
		e.explainQuery(query, args...)
	}

	res := <-results
	return res.result, res.err
}

func (e *explainConn) explainQuery(query string, args ...interface{}) {
	logger := e.logger.WithData(lager.Data{
		"query": query,
		"args":  args,
	})

	rows, err := e.Conn.Query("EXPLAIN "+query, args...)
	if err != nil {
		logger.Error("failed-to-explain", err)
		return
	}

	message := &bytes.Buffer{}
	var line string

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&line)
		if err != nil {
			logger.Error("failed-to-scan", err)
			return
		}

		message.WriteString(line)
		message.WriteString("\n")
	}

	if rows.Err() != nil {
		logger.Error("failed-on-final-iteration", err)
		return
	}

	logger.Debug("slow-query", lager.Data{
		"explained-plan": message.String(),
	})
}
