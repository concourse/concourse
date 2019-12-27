package tsacmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/tsa"
	"golang.org/x/crypto/ssh"
)

const maxForwards = 2

type server struct {
	logger            lager.Logger
	atcEndpointPicker tsa.EndpointPicker
	heartbeatInterval time.Duration
	cprInterval       time.Duration
	forwardHost       string
	config            *ssh.ServerConfig
	httpClient        *http.Client
	sessionTeam       *sessionTeam
}

type sessionTeam struct {
	sessionTeams map[string]string
	lock         *sync.RWMutex
}

func (s *sessionTeam) AuthorizeTeam(sessionID, team string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.sessionTeams[sessionID] = team
}

func (s *sessionTeam) IsNotAuthorized(sessionID, team string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	t, found := s.sessionTeams[sessionID]

	return found && t != team
}

func (s *sessionTeam) AuthorizedTeamFor(sessionID string) string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.sessionTeams[sessionID]
}

type ConnState struct {
	Team string

	ForwardedTCPIPs <-chan ForwardedTCPIP
}

type ForwardedTCPIP struct {
	Logger lager.Logger

	BindAddr  string
	BoundPort uint32

	Drain chan<- struct{}

	wg *sync.WaitGroup
}

func (forward ForwardedTCPIP) Wait() {
	forward.Logger.Debug("draining")
	forward.wg.Wait()
	forward.Logger.Debug("drained")
}

func (server *server) Serve(listener net.Listener) {
	for {
		c, err := listener.Accept()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				server.logger.Error("failed-to-accept", err)
			}

			return
		}

		logger := server.logger.Session("connection", lager.Data{
			"remote": c.RemoteAddr().String(),
		})

		go server.handshake(logger, c)
	}
}

func (server *server) handshake(logger lager.Logger, netConn net.Conn) {
	conn, chans, reqs, err := ssh.NewServerConn(netConn, server.config)
	if err != nil {
		logger.Info("handshake-failed", lager.Data{"error": err.Error()})
		return
	}

	defer conn.Close()

	ctx, cancel := context.WithCancel(lagerctx.NewContext(context.Background(), logger))
	defer cancel()

	sessionID := string(conn.SessionID())

	forwardedTCPIPs := make(chan ForwardedTCPIP, maxForwards)
	go server.handleForwardRequests(ctx, conn, reqs, forwardedTCPIPs)

	state := ConnState{
		Team: server.sessionTeam.AuthorizedTeamFor(sessionID),

		ForwardedTCPIPs: forwardedTCPIPs,
	}

	chansGroup := new(sync.WaitGroup)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			logger.Info("rejecting-unknown-channel-type", lager.Data{
				"type": newChannel.ChannelType(),
			})

			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			logger.Error("failed-to-accept-channel", err)
			return
		}

		chansGroup.Add(1)
		go server.handleChannel(logger.Session("channel"), chansGroup, channel, requests, state)
	}

	chansGroup.Wait()
}

type signalMsg struct {
	Signal string
}

func (server *server) handleChannel(
	logger lager.Logger,
	chansGroup *sync.WaitGroup,
	channel ssh.Channel,
	requests <-chan *ssh.Request,
	state ConnState,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer chansGroup.Done()
	defer channel.Close()

	execExited := make(chan error, 1)

	for {
		select {
		case req, ok := <-requests:
			if !ok {
				return
			}

			logger.Debug("channel-request", lager.Data{
				"type": req.Type,
			})

			switch req.Type {
			case "signal":
				req.Reply(true, nil)

				var sig signalMsg
				err := ssh.Unmarshal(req.Payload, &sig)
				if err != nil {
					logger.Error("malformed-signal", err)
					req.Reply(false, nil)
					continue
				}

				logger.Debug("received-signal", lager.Data{
					"signal": sig,
				})

				cancel()

			case "exec":
				var request execRequest
				err := ssh.Unmarshal(req.Payload, &request)
				if err != nil {
					logger.Error("malformed-exec-request", err)
					req.Reply(false, nil)
					return
				}

				workerRequest, command, err := server.parseRequest(request.Command)
				if err != nil {
					fmt.Fprintf(channel, "invalid command: %s", err)
					req.Reply(false, nil)
					continue
				}

				req.Reply(true, nil)

				cmdLogger := logger.Session("command", lager.Data{
					"command": command,
				})

				go func() {
					execExited <- workerRequest.Handle(lagerctx.NewContext(ctx, cmdLogger), state, channel)
				}()

			default:
				logger.Info("rejecting")
				req.Reply(false, nil)
				continue
			}

		case err := <-execExited:
			req := exitStatusRequest{0}

			if err != nil {
				logger.Error("exited-with-error", err)
				req.ExitStatus = 1
			} else {
				logger.Debug("exited-successfully")
			}

			_, err = channel.SendRequest("exit-status", false, ssh.Marshal(req))
			if err != nil {
				logger.Error("failed-to-send-exit-status", err)
			}

			// RFC 4254: "The channel needs to be closed with SSH_MSG_CHANNEL_CLOSE after
			// this message."
			err = channel.Close()
			if err != nil {
				logger.Error("failed-to-close-channel", err)
			} else {
				logger.Debug("closed-channel")
			}
		}
	}
}

func (server *server) handleForwardRequests(
	ctx context.Context,
	conn *ssh.ServerConn,
	reqs <-chan *ssh.Request,
	forwardedTCPIPs chan<- ForwardedTCPIP,
) {
	logger := lagerctx.FromContext(ctx)

	var forwardedThings int

	for r := range reqs {
		reqLog := logger.Session("request", lager.Data{
			"type": r.Type,
		})

		switch r.Type {
		case "tcpip-forward":
			forwardedThings++

			if forwardedThings > maxForwards {
				reqLog.Info("rejecting-extra-forward-request")
				r.Reply(false, nil)
				continue
			}

			var req tcpipForwardRequest
			err := ssh.Unmarshal(r.Payload, &req)
			if err != nil {
				reqLog.Error("malformed-tcpip-request", err)
				r.Reply(false, nil)
				continue
			}

			bindAddr := net.JoinHostPort(req.BindIP, fmt.Sprintf("%d", req.BindPort))

			listener, err := net.Listen("tcp", "0.0.0.0:0")
			if err != nil {
				reqLog.Error("failed-to-listen", err)
				r.Reply(false, nil)
				continue
			}

			defer listener.Close()

			_, port, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				r.Reply(false, nil)
				continue
			}

			var res tcpipForwardResponse
			_, err = fmt.Sscanf(port, "%d", &res.BoundPort)
			if err != nil {
				r.Reply(false, nil)
				continue
			}

			reqLog = reqLog.WithData(lager.Data{
				"addr":           listener.Addr().String(),
				"requested-addr": bindAddr,
			})

			reqLog.Debug("listening")

			forPort := req.BindPort
			if forPort == 0 {
				forPort = res.BoundPort
			}

			drain := make(chan struct{})
			wait := new(sync.WaitGroup)

			wait.Add(1)
			go server.forwardTCPIP(lagerctx.NewContext(ctx, reqLog), drain, wait, conn, listener, req.BindIP, forPort)

			forwardedTCPIPs <- ForwardedTCPIP{
				Logger: reqLog,

				BindAddr:  bindAddr,
				BoundPort: res.BoundPort,

				Drain: drain,

				wg: wait,
			}

			r.Reply(true, ssh.Marshal(res))

		default:
			// OpenSSH sends keepalive@openssh.com, but there may be other clients;
			// just check for 'keepalive'
			if strings.Contains(r.Type, "keepalive") {
				reqLog.Debug("keepalive")
				r.Reply(true, nil)
			} else {
				reqLog.Info("ignoring")
				r.Reply(false, nil)
			}
		}
	}
}

func (server *server) forwardTCPIP(
	ctx context.Context,
	drain <-chan struct{},
	connsWg *sync.WaitGroup,
	conn *ssh.ServerConn,
	listener net.Listener,
	forwardIP string,
	forwardPort uint32,
) {
	defer connsWg.Done()

	logger := lagerctx.FromContext(ctx)

	done := make(chan struct{})
	defer close(done)

	interrupted := false
	go func() {
		select {
		case <-drain:
			logger.Debug("draining")
			interrupted = true
			listener.Close()
		case <-done:
			logger.Debug("done")
		}
	}()

	for {
		localConn, err := listener.Accept()
		if err != nil {
			if !interrupted {
				logger.Error("failed-to-accept", err)
			}

			break
		}

		connsWg.Add(1)

		go func() {
			defer connsWg.Done()

			forwardLocalConn(
				lagerctx.NewContext(ctx, logger.Session("forward-conn")),
				localConn,
				conn,
				forwardIP,
				forwardPort,
			)
		}()
	}
}

func forwardLocalConn(ctx context.Context, localConn net.Conn, conn *ssh.ServerConn, forwardIP string, forwardPort uint32) {
	logger := lagerctx.FromContext(ctx)

	defer localConn.Close()

	var req forwardTCPIPChannelRequest
	req.ForwardIP = forwardIP
	req.ForwardPort = forwardPort

	host, port, err := net.SplitHostPort(localConn.RemoteAddr().String())
	if err != nil {
		logger.Error("failed-to-split-host-port", err)
		return
	}

	req.OriginIP = host

	_, err = fmt.Sscanf(port, "%d", &req.OriginPort)
	if err != nil {
		logger.Error("failed-to-parse-port", err)
		return
	}

	channel, reqs, err := conn.OpenChannel("forwarded-tcpip", ssh.Marshal(req))
	if err != nil {
		logger.Error("failed-to-open-channel", err)
		return
	}

	defer channel.Close()

	go ssh.DiscardRequests(reqs)

	numPipes := 2
	wait := make(chan struct{}, numPipes)

	pipe := func(to io.WriteCloser, from io.ReadCloser) {
		// if either end breaks, close both ends to ensure they're both unblocked,
		// otherwise io.Copy can block forever if e.g. reading after write end has
		// gone away
		defer to.Close()
		defer from.Close()
		defer func() {
			wait <- struct{}{}
		}()

		io.Copy(to, from)
	}

	go pipe(localConn, channel)
	go pipe(channel, localConn)

	done := 0
dance:
	for {
		select {
		case <-wait:
			done++
			if done == numPipes {
				break dance
			}

			logger.Debug("tcpip-io-complete")
		case <-ctx.Done():
			logger.Debug("tcpip-io-interrupted")
			break dance
		}
	}
}

func (server *server) parseRequest(cli string) (request, string, error) {
	argv := strings.Split(cli, " ")

	command := argv[0]
	args := argv[1:]

	var req request
	switch command {
	case tsa.ForwardWorker:
		var fs = flag.NewFlagSet(command, flag.ContinueOnError)

		var garden = fs.String("garden", "", "garden address to forward")
		var baggageclaim = fs.String("baggageclaim", "", "baggageclaim address to forward")

		err := fs.Parse(args)
		if err != nil {
			return nil, "", err
		}

		req = forwardWorkerRequest{
			server: server,

			gardenAddr:       *garden,
			baggageclaimAddr: *baggageclaim,
		}
	case tsa.LandWorker:
		req = landWorkerRequest{
			server: server,
		}
	case tsa.RetireWorker:
		req = retireWorkerRequest{
			server: server,
		}
	case tsa.DeleteWorker:
		req = deleteWorkerRequest{
			server: server,
		}
	case tsa.SweepContainers:
		req = sweepContainersRequest{
			server: server,
		}
	case tsa.ReportContainers:
		req = reportContainersRequest{
			server:           server,
			containerHandles: args,
		}
	case tsa.SweepVolumes:
		req = sweepVolumesRequest{
			server: server,
		}
	case tsa.ReportVolumes:
		req = reportVolumesRequest{
			server:        server,
			volumeHandles: args,
		}
	default:
		return nil, "", fmt.Errorf("unknown command: %s", command)
	}

	return req, command, nil
}
