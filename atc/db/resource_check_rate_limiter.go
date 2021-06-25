package db

import (
	"context"
	"fmt"
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

		// The first time we call Wait, we will properly update the limit.
		// This is just to avoid dealing with the limiter not existing.
		limiter.checkLimiter = rate.NewLimiter(rate.Inf, 1)
	}

	return limiter
}

func (limiter *ResourceCheckRateLimiter) Wait(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	if limiter.refreshLimiter != nil && limiter.refreshLimiter.AllowN(limiter.clock.Now(), 1) {
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
	logger.Debug("resource-rate-limit-exceeded", lager.Data{"waiting-for": delay.String()})

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
	return limiter.checkLimiter.Limit()
}

func (limiter *ResourceCheckRateLimiter) refreshCheckLimiter() error {
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

	if limit != limiter.checkLimiter.Limit() {
		limiter.checkLimiter.SetLimit(limit)
	}

	return nil
}
