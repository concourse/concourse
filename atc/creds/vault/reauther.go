package vault

import (
	"code.cloudfoundry.org/lager"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
)

// An Auther is anything which needs to be logged in and then have
// that login renewed on a regulary basis.
type Auther interface {
	Login() (time.Duration, error)
	Renew() (time.Duration, error)
}

// The ReAuther runs the authorization loop (login, renew) and retries
// using a bounded exponential backoff strategy. If maxTTL is set, a
// new login will be done _regardless_ of the available leaseDuration.
type ReAuther struct {
	auther Auther
	base   time.Duration
	max    time.Duration
	maxTTL time.Duration

	loggedIn     chan struct{}
	loggedInOnce *sync.Once

	closedCh chan struct{}

	logger lager.Logger
}

// NewReAuther with a retry time and a max retry time.
func NewReAuther(logger lager.Logger, auther Auther, maxTTL, retry, max time.Duration) *ReAuther {
	ra := &ReAuther{
		auther: auther,
		base:   retry,
		max:    max,
		maxTTL: maxTTL,

		loggedIn:     make(chan struct{}, 1),
		loggedInOnce: &sync.Once{},

		closedCh: make(chan struct{}, 1),

		logger: logger,
	}

	go ra.authLoop()

	return ra
}

// LoggedIn will receive a signal after every login. Multiple logins
// may result in a single signal as this channel is not blocked.
func (ra *ReAuther) LoggedIn() <-chan struct{} {
	return ra.loggedIn
}

func (ra *ReAuther) Close() {
	ra.logger.Debug("vault-reauther-close")
	close(ra.closedCh)
}

// we can't renew a secret that has exceeded it's maxTTL or it's lease
func (ra *ReAuther) renewable(leaseEnd, tokenEOL time.Time) bool {
	now := time.Now()

	if ra.maxTTL != 0 && now.After(tokenEOL) {
		// token has exceeded the configured max TTL
		return false
	}

	if now.After(leaseEnd) {
		// token has exceeded its lease
		return false
	}

	return true
}

// sleep until the tokenEOl or half the lease duration
func (ra *ReAuther) sleep(leaseEnd, tokenEOL time.Time) {
	if ra.maxTTL != 0 && leaseEnd.After(tokenEOL) {
		time.Sleep(time.Until(tokenEOL))
	} else {
		time.Sleep(time.Until(leaseEnd) / 2)
	}
}

func (ra *ReAuther) closed() bool {
	select {
	case <-ra.closedCh:
		ra.logger.Debug("vault-reauther-closed")
		return true
	default: // default clause makes above channel non-blocking.
	}
	return false
}

func (ra *ReAuther) authLoop() {
	var tokenEOL, leaseEnd time.Time

	ra.logger.Debug("vault-reauther-started")
	defer ra.logger.Debug("vault-reauther-terminated")

	for {
		exp := backoff.NewExponentialBackOff()
		exp.InitialInterval = ra.base
		exp.MaxInterval = ra.max
		exp.MaxElapsedTime = 0
		exp.Reset()

		for {
			if ra.closed() {
				return
			}

			lease, err := ra.auther.Login()
			if err != nil {
				time.Sleep(exp.NextBackOff())
				continue
			}

			exp.Reset()

			ra.loggedInOnce.Do(func() {
				close(ra.loggedIn)
			})

			now := time.Now()
			tokenEOL = now.Add(ra.maxTTL)
			leaseEnd = now.Add(lease)
			ra.sleep(leaseEnd, tokenEOL)

			break
		}

		for {
			if ra.closed() {
				return
			}

			if !ra.renewable(leaseEnd, tokenEOL) {
				break
			}

			lease, err := ra.auther.Renew()
			if err != nil {
				time.Sleep(exp.NextBackOff())
				continue
			}

			exp.Reset()

			leaseEnd = time.Now().Add(lease)
			ra.sleep(leaseEnd, tokenEOL)
		}
	}
}
