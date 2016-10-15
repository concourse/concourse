package tsacmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/tsa"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/rata"
	"golang.org/x/crypto/ssh"
)

type registrarSSHServer struct {
	logger            lager.Logger
	atcEndpoint       *rata.RequestGenerator
	tokenGenerator    tsa.TokenGenerator
	heartbeatInterval time.Duration
	cprInterval       time.Duration
	forwardHost       string
	config            *ssh.ServerConfig
	httpClient        *http.Client
	sessionTeam       sessionTeam
}

type sessionTeam map[string]string

type forwardedTCPIP struct {
	bindAddr  string
	process   ifrit.Process
	boundPort uint32
}

func (server *registrarSSHServer) Serve(listener net.Listener) {
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

func (server *registrarSSHServer) handshake(logger lager.Logger, netConn net.Conn) {
	conn, chans, reqs, err := ssh.NewServerConn(netConn, server.config)
	if err != nil {
		logger.Info("handshake-failed", lager.Data{"error": err.Error()})
		return
	}

	defer conn.Close()

	forwardedTCPIPs := make(chan forwardedTCPIP, 2)
	go server.handleForwardRequests(logger, conn, reqs, forwardedTCPIPs)

	var processes []ifrit.Process
	var process ifrit.Process

	sessionID := string(conn.SessionID())

	// ensure processes get cleaned up
	defer func() {
		cleanupLog := logger.Session("cleanup")

		for _, p := range processes {
			cleanupLog.Debug("interrupting")

			p.Signal(os.Interrupt)
		}

		for _, p := range processes {
			err := <-p.Wait()
			if err != nil {
				cleanupLog.Error("process-exited-with-failure", err)
			} else {
				cleanupLog.Debug("process-exited-successfully")
			}
		}
	}()

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

		defer channel.Close()

		for req := range requests {
			logger.Info("channel-request", lager.Data{
				"type": req.Type,
			})

			if req.Type != "exec" {
				logger.Info("rejecting")
				req.Reply(false, nil)
				continue
			}

			var request execRequest
			err = ssh.Unmarshal(req.Payload, &request)
			if err != nil {
				logger.Error("malformed-exec-request", err)
				req.Reply(false, nil)
				return
			}

			workerRequest, err := parseRequest(request.Command)
			if err != nil {
				fmt.Fprintf(channel, "invalid command: %s", err)
				req.Reply(false, nil)
				continue
			}

			switch r := workerRequest.(type) {
			case registerWorkerRequest:
				logger := logger.Session("register-worker")

				req.Reply(true, nil)

				process, err = server.continuouslyRegisterWorkerDirectly(logger, channel, sessionID)
				if err != nil {
					logger.Error("failed-to-register", err)
					return
				}

				processes = append(processes, process)

				err = conn.Wait()
				logger.Error("connection-closed", err)

			case forwardWorkerRequest:
				logger := logger.Session("forward-worker")

				req.Reply(true, nil)

				forwards := map[string]forwardedTCPIP{}

				for i := 0; i < r.expectedForwards(); i++ {
					select {
					case forwarded := <-forwardedTCPIPs:
						logger.Info("forwarded-tcpip", lager.Data{
							"bound-port": forwarded.boundPort,
						})

						processes = append(processes, forwarded.process)

						forwards[forwarded.bindAddr] = forwarded

					case <-time.After(10 * time.Second): // todo better?
						logger.Info("never-forwarded-tcpip")
					}
				}

				switch len(forwards) {
				case 0:
					fmt.Fprintf(channel, "requested forwarding but no forwards given\n")
					return

				case 1:
					for _, gardenForward := range forwards {
						process, err = server.continuouslyRegisterForwardedWorker(
							logger,
							channel,
							gardenForward.boundPort,
							0,
							sessionID,
						)
						if err != nil {
							logger.Error("failed-to-register", err)
							return
						}

						processes = append(processes, process)

						break
					}

				case 2:
					gardenForward, found := forwards[r.gardenAddr]
					if !found {
						fmt.Fprintf(channel, "garden address %s not found in forwards\n", r.gardenAddr)
						return
					}

					baggageclaimForward, found := forwards[r.baggageclaimAddr]
					if !found {
						fmt.Fprintf(channel, "baggageclaim address %s not found in forwards\n", r.gardenAddr)
						return
					}

					process, err = server.continuouslyRegisterForwardedWorker(
						logger,
						channel,
						gardenForward.boundPort,
						baggageclaimForward.boundPort,
						sessionID,
					)
					if err != nil {
						logger.Error("failed-to-register", err)
						return
					}

					processes = append(processes, process)
				}

				err = conn.Wait()
				logger.Error("connection-closed", err)

			default:
				logger.Info("invalid-command", lager.Data{
					"command": request.Command,
				})

				req.Reply(false, nil)
			}
		}
	}
}

func (server *registrarSSHServer) continuouslyRegisterWorkerDirectly(
	logger lager.Logger,
	channel ssh.Channel,
	sessionID string,
) (ifrit.Process, error) {
	logger.Session("start")
	defer logger.Session("done")

	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return nil, err
	}

	err = server.validateWorkerTeam(logger, sessionID, worker)
	if err != nil {
		return nil, err
	}

	return server.heartbeatWorker(logger, worker, channel), nil
}

func (server *registrarSSHServer) validateWorkerTeam(
	logger lager.Logger,
	sessionID string,
	worker atc.Worker,
) error {
	connTeam := server.sessionTeam[sessionID]
	if connTeam != "" && worker.Team != connTeam {
		logger.Info("worker-not-allowed", lager.Data{
			"connection-team": connTeam,
			"channel-team":    worker.Team,
		})
		return errors.New("worker-not-allowed-to-team")
	}

	return nil
}

func (server *registrarSSHServer) continuouslyRegisterForwardedWorker(
	logger lager.Logger,
	channel ssh.Channel,
	gardenPort uint32,
	baggageclaimPort uint32,
	sessionID string,
) (ifrit.Process, error) {
	logger.Session("start")
	defer logger.Session("done")

	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return nil, err
	}

	err = server.validateWorkerTeam(logger, sessionID, worker)
	if err != nil {
		return nil, err
	}

	worker.GardenAddr = fmt.Sprintf("%s:%d", server.forwardHost, gardenPort)

	if baggageclaimPort != 0 {
		worker.BaggageclaimURL = fmt.Sprintf("http://%s:%d", server.forwardHost, baggageclaimPort)
	}

	return server.heartbeatWorker(logger, worker, channel), nil
}

func (server *registrarSSHServer) heartbeatWorker(logger lager.Logger, worker atc.Worker, channel ssh.Channel) ifrit.Process {
	return ifrit.Background(tsa.NewHeartbeater(
		logger,
		clock.NewClock(),
		server.heartbeatInterval,
		server.cprInterval,
		gclient.New(gconn.NewWithDialerAndLogger(keepaliveDialerFactory("tcp", worker.GardenAddr), logger.Session("garden-connection"))),
		server.atcEndpoint,
		server.tokenGenerator,
		worker,
		channel,
	))
}

func (server *registrarSSHServer) handleForwardRequests(
	logger lager.Logger,
	conn *ssh.ServerConn,
	reqs <-chan *ssh.Request,
	forwardedTCPIPs chan<- forwardedTCPIP,
) {
	var forwardedThings int

	for r := range reqs {
		switch r.Type {
		case "tcpip-forward":
			logger := logger.Session("tcpip-forward")

			forwardedThings++

			if forwardedThings > 2 {
				logger.Info("rejecting-extra-forward-request")
				r.Reply(false, nil)
				continue
			}

			var req tcpipForwardRequest
			err := ssh.Unmarshal(r.Payload, &req)
			if err != nil {
				logger.Error("malformed-tcpip-request", err)
				r.Reply(false, nil)
				continue
			}

			listener, err := net.Listen("tcp", "0.0.0.0:0")
			if err != nil {
				logger.Error("failed-to-listen", err)
				r.Reply(false, nil)
				continue
			}

			defer listener.Close()

			bindAddr := net.JoinHostPort(req.BindIP, fmt.Sprintf("%d", req.BindPort))

			logger.Info("forwarding-tcpip", lager.Data{
				"requested-bind-addr": bindAddr,
			})

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

			forPort := req.BindPort
			if forPort == 0 {
				forPort = res.BoundPort
			}

			process := server.forwardTCPIP(logger, conn, listener, req.BindIP, forPort)

			forwardedTCPIPs <- forwardedTCPIP{
				bindAddr:  fmt.Sprintf("%s:%d", req.BindIP, req.BindPort),
				boundPort: res.BoundPort,
				process:   process,
			}

			r.Reply(true, ssh.Marshal(res))

		default:
			// OpenSSH sends keepalive@openssh.com, but there may be other clients;
			// just check for 'keepalive'
			if strings.Contains(r.Type, "keepalive") {
				logger.Info("keepalive", lager.Data{"type": r.Type})
				r.Reply(true, nil)
			} else {
				logger.Info("ignoring-request", lager.Data{"type": r.Type})
				r.Reply(false, nil)
			}
		}
	}
}

func (server *registrarSSHServer) forwardTCPIP(
	logger lager.Logger,
	conn *ssh.ServerConn,
	listener net.Listener,
	forwardIP string,
	forwardPort uint32,
) ifrit.Process {
	return ifrit.Background(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		go func() {
			<-signals

			listener.Close()
		}()

		close(ready)

		for {
			localConn, err := listener.Accept()
			if err != nil {
				logger.Error("failed-to-accept", err)
				break
			}

			go forwardLocalConn(logger, localConn, conn, forwardIP, forwardPort)
		}

		return nil
	}))
}

func forwardLocalConn(logger lager.Logger, localConn net.Conn, conn *ssh.ServerConn, forwardIP string, forwardPort uint32) {
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

	go func() {
		for r := range reqs {
			logger.Info("ignoring-request", lager.Data{
				"type": r.Type,
			})

			r.Reply(false, nil)
		}
	}()

	wg := new(sync.WaitGroup)

	pipe := func(to io.WriteCloser, from io.ReadCloser) {
		// if either end breaks, close both ends to ensure they're both unblocked,
		// otherwise io.Copy can block forever if e.g. reading after write end has
		// gone away
		defer to.Close()
		defer from.Close()
		defer wg.Done()

		io.Copy(to, from)
	}

	wg.Add(1)
	go pipe(localConn, channel)

	wg.Add(1)
	go pipe(channel, localConn)

	wg.Wait()
}

func keepaliveDialerFactory(network string, address string) gconn.DialerFunc {
	return func(string, string) (net.Conn, error) {
		return keepaliveDialer(network, address)
	}
}
