package db

import (
	"context"
	"fmt"
)

type BuildEventIDAllocator interface {
	Initialize(ctx context.Context, buildID int) error
	Finalize(ctx context.Context, buildID int) error

	Allocate(ctx context.Context, buildID int, count int) (IDBlock, error)
}

type IDBlock interface {
	Next() (EventKey, bool)
}

type rawIDBlock struct {
	ids   []EventKey
	index int
}

func (r *rawIDBlock) Next() (EventKey, bool) {
	if r.index >= len(r.ids) {
		return 0, false
	}
	r.index++
	return r.ids[r.index-1], true
}

type buildEventIDAllocator struct {
	conn Conn
}

func NewBuildEventIDAllocator(conn Conn) BuildEventIDAllocator {
	return &buildEventIDAllocator{conn}
}

func (b buildEventIDAllocator) Initialize(ctx context.Context, buildID int) error {
	_, err := b.conn.ExecContext(ctx, fmt.Sprintf(`CREATE SEQUENCE %s MINVALUE 0`, buildEventsSeq(buildID)))
	if err != nil {
		return fmt.Errorf("create sequence: %w", err)
	}
	return nil
}

func (b buildEventIDAllocator) Finalize(ctx context.Context, buildID int) error {
	_, err := b.conn.ExecContext(ctx, fmt.Sprintf(`DROP SEQUENCE %s`, buildEventsSeq(buildID)))
	if err != nil {
		return fmt.Errorf("drop sequence: %w", err)
	}
	return nil
}

func (b buildEventIDAllocator) Allocate(ctx context.Context, buildID int, count int) (IDBlock, error) {
	if count == 0 {
		return nil, nil
	}
	rows, err := b.conn.QueryContext(ctx,
		fmt.Sprintf(`SELECT nextval('%s') FROM generate_series(1, %d)`, buildEventsSeq(buildID), count))
	if err != nil {
		return nil, fmt.Errorf("generate ids: %w", err)
	}
	defer rows.Close()
	ids := make([]EventKey, 0, count)
	for rows.Next() {
		var id EventKey
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id: %w", err)
		}
		ids = append(ids, id)
	}
	return &rawIDBlock{ids: ids}, nil
}

func buildEventsSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}
