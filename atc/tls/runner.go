package tls

import (
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
)

type ConfigReloader func() (*tls.Config, error)

type ReloadableHTTPSListener struct {
	address     string
	handler     http.Handler
	tlsConfig   *tls.Config
	active      ifrit.Process
	reloader    ConfigReloader
	Interrupter chan os.Signal
	logger      lager.Logger
	activeMu    *sync.Mutex
}

func (r *ReloadableHTTPSListener) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	r.logger.Info("start", lager.Data{"listen-addr": r.address})
	if err := r.Start(); err != nil {
		return err
	}

	close(ready)

	for {
		select {
		case <-r.Interrupter:
			r.logger.Info("reload-requested")
			if err := r.Restart(); err != nil {
				return err
			}

			continue
		case <-signals:
			r.Stop()
			return nil
		}
	}
}

func (r *ReloadableHTTPSListener) Start() error {
	if r.isStarted() {
		return errors.New("tls listener already started")
	}

	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	r.active = ifrit.Invoke(http_server.NewTLSServer(r.address, r.handler, r.tlsConfig))

	select {
	case <-r.active.Ready():
		return nil
	case err := <-r.active.Wait():
		return err
	}
}

func (r *ReloadableHTTPSListener) Stop() error {
	var err error
	r.activeMu.Lock()
	defer r.activeMu.Unlock()
	if r.active != nil {
		r.active.Signal(os.Interrupt)
		err = <-r.active.Wait()
	}

	r.active = nil
	return err
}

func (r *ReloadableHTTPSListener) Restart() error {
	var err error

	r.Stop()
	r.tlsConfig, err = r.reloader()
	if err != nil {
		return err
	}

	return r.Start()
}

func (r *ReloadableHTTPSListener) isStarted() bool {
	r.activeMu.Lock()
	defer r.activeMu.Unlock()

	return r.active != nil
}

func NewReloadableListener(
	address string, handler http.Handler, config *tls.Config,
	reloader ConfigReloader, logger lager.Logger,
) *ReloadableHTTPSListener {
	r := &ReloadableHTTPSListener{
		address:     address,
		handler:     handler,
		tlsConfig:   config,
		reloader:    reloader,
		logger:      logger,
		Interrupter: make(chan os.Signal, 1),
		activeMu:    new(sync.Mutex),
	}

	signal.Notify(r.Interrupter, syscall.SIGHUP)

	return r
}
