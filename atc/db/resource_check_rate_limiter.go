package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	sq "github.com/Masterminds/squirrel"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"golang.org/x/time/rate"
)

type ResourceCheckRateLimiter struct {
	checkLimiter *rate.Limiter

	refreshConn    Conn
	checkInterval  time.Duration
	refreshLimiter *rate.Limiter

	clock clock.Clock
	mut   *sync.Mutex
}

func NewResourceCheckRateLimiter(
	checksPerSecond rate.Limit,
	checkInterval time.Duration,
	refreshConn Conn,
	refreshInterval time.Duration,
	clock clock.Clock,
) *ResourceCheckRateLimiter {
	limiter := &ResourceCheckRateLimiter{
		clock: clock,
		mut:   new(sync.Mutex),
	}

	if checksPerSecond < 0 {
		checksPerSecond = rate.Inf
	}

	if checksPerSecond != 0 {
		limiter.checkLimiter = rate.NewLimiter(checksPerSecond, 1)
	} else {
		limiter.checkInterval = checkInterval
		limiter.refreshConn = refreshConn
		limiter.refreshLimiter = rate.NewLimiter(rate.Every(refreshInterval), 1)
	}

	return limiter
}

func (limiter *ResourceCheckRateLimiter) Wait(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("rate-limiter")

	limiter.mut.Lock()
	defer limiter.mut.Unlock()

	if limiter.refreshLimiter != nil && limiter.refreshLimiter.AllowN(limiter.clock.Now(), 1) {
		err := limiter.refreshCheckLimiter(logger)
		if err != nil {
			return fmt.Errorf("refresh: %w", err)
		}
	}

	reservation := limiter.checkLimiter.ReserveN(limiter.clock.Now(), 1)

	delay := reservation.DelayFrom(limiter.clock.Now())
	logger.Debug("reserved", lager.Data{"reservation": reservation, "delay": delay})
	if delay == 0 {
		return nil
	}

	timer := limiter.clock.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C():
		logger.Debug("waited")
		return nil
	case <-ctx.Done():
		logger.Debug("canceled")
		reservation.Cancel()
		return ctx.Err()
	}
}

func (limiter *ResourceCheckRateLimiter) Limit() rate.Limit {
	limiter.mut.Lock()
	defer limiter.mut.Unlock()

	return limiter.checkLimiter.Limit()
}

func (limiter *ResourceCheckRateLimiter) refreshCheckLimiter(logger lager.Logger) error {
	var count int
	err := psql.Select("COUNT(*)").
		From("resources r").
		Join("pipelines p ON p.id = r.pipeline_id").
		Where(sq.Eq{
			"r.active": true,
			"p.paused": false,
		}).
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
	logger.Debug("refresh", lager.Data{"count": count, "limit": limit})

	if limiter.checkLimiter == nil {
		limiter.checkLimiter = rate.NewLimiter(limit, 1)
	} else if limit != limiter.checkLimiter.Limit() {
		limiter.checkLimiter.SetLimit(limit)
	}

	return nil
}
