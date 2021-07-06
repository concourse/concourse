package gc

import (
	"context"
	"github.com/concourse/concourse/atc/component"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

type accessTokensCollector struct {
	lifecycle db.AccessTokenLifecycle
	leeway    time.Duration
}

func NewAccessTokensCollector(lifecycle db.AccessTokenLifecycle, leeway time.Duration) *accessTokensCollector {
	return &accessTokensCollector{
		lifecycle: lifecycle,
		leeway:    leeway,
	}
}

func (c *accessTokensCollector) Run(ctx context.Context, _ string) (component.RunResult, error) {
	logger := lagerctx.FromContext(ctx).Session("access-tokens-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	_, err := c.lifecycle.RemoveExpiredAccessTokens(c.leeway)
	if err != nil {
		logger.Error("failed-to-remove-expired-access-tokens", err)
		return nil, err
	}

	return nil, nil
}
