package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"golang.org/x/time/rate"
)

type ResourceCheckRateLimiter struct {
	clock clock.Clock

	checkInterval time.Duration
	checkLimiter  *rate.Limiter

	refreshConn    Conn
	refreshLimiter *rate.Limiter

	mut *sync.Mutex
}

func NewResourceCheckRateLimiter(
	refreshConn Conn,
	checkInterval time.Duration,
	refreshInterval time.Duration,
	clock clock.Clock,
) *ResourceCheckRateLimiter {
	return &ResourceCheckRateLimiter{
		clock: clock,

		checkInterval: checkInterval,
		checkLimiter:  nil,

		refreshConn:    refreshConn,
		refreshLimiter: rate.NewLimiter(rate.Every(refreshInterval), 1),

		mut: new(sync.Mutex),
	}
}

func (limiter *ResourceCheckRateLimiter) Wait(ctx context.Context) error {
	limiter.mut.Lock()
	defer limiter.mut.Unlock()

	if limiter.refreshLimiter.AllowN(limiter.clock.Now(), 1) {
		err := limiter.refreshCheckLimiter()
		if err != nil {
			return fmt.Errorf("refresh: %w", err)
		}
	}

	reservation := limiter.checkLimiter.ReserveN(limiter.clock.Now(), 1)

	delay := reservation.DelayFrom(limiter.clock.Now())
	if delay == 0 {
		return nil
	}

	timer := limiter.clock.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C():
		return nil
	case <-ctx.Done():
		reservation.Cancel()
		return ctx.Err()
	}
}

func (limiter *ResourceCheckRateLimiter) Limit() rate.Limit {
	limiter.mut.Lock()
	defer limiter.mut.Unlock()

	return limiter.checkLimiter.Limit()
}

func (limiter *ResourceCheckRateLimiter) refreshCheckLimiter() error {
	var count int
	err := psql.Select("COUNT(id)").
		From("resource_config_scopes").
		RunWith(limiter.refreshConn).
		QueryRow().
		Scan(&count)
	if err != nil {
		return err
	}

	limit := rate.Limit(float64(count) / limiter.checkInterval.Seconds())
	if count == 0 {
		// don't bother waiting if there aren't any checkables
		limit = rate.Inf
	}

	if limiter.checkLimiter == nil {
		limiter.checkLimiter = rate.NewLimiter(limit, 1)
	} else if limit != limiter.checkLimiter.Limit() {
		limiter.checkLimiter.SetLimit(limit)
	}

	return nil
}
