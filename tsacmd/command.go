package tsacmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"

	"golang.org/x/crypto/ssh"

	"github.com/concourse/flag"
	"github.com/concourse/tsa"
	"github.com/concourse/tsa/tsaflags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

type TSACommand struct {
	Logger flag.Lager

	BindIP        flag.IP `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for SSH."`
	BindPort      uint16  `long:"bind-port" default:"2222"    description:"Port on which to listen for SSH."`
	DebugBindPort uint16  `long:"bind-debug-port" default:"8089"    description:"Port on which to listen for TSA pprof server."`
	PeerIP        string  `long:"peer-ip" required:"true" description:"IP address of this TSA, reachable by the ATCs. Used for forwarded worker addresses."`

	HostKey            *flag.PrivateKey         `long:"host-key"        required:"true" description:"Path to private key to use for the SSH server."`
	AuthorizedKeys     flag.AuthorizedKeys      `long:"authorized-keys" required:"true" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`
	TeamAuthorizedKeys []tsaflags.InputPairFlag `long:"team-authorized-keys" value-name:"NAME=PATH" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`

	ATCURLs []flag.URL `long:"atc-url" required:"true" description:"ATC API endpoints to which workers will be registered."`

	SessionSigningKey *flag.PrivateKey `long:"session-signing-key" required:"true" description:"Path to private key to use when signing tokens in reqests to the ATC during registration."`

	HeartbeatInterval time.Duration `long:"heartbeat-interval" default:"30s" description:"interval on which to heartbeat workers to the ATC"`
}

func (cmd *TSACommand) debugBindAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", cmd.DebugBindPort)
}

type TeamAuthKeys struct {
	Team     string
	AuthKeys []ssh.PublicKey
}

func (cmd *TSACommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	tsaServerMember := grouper.Member{
		Name:   "tsa-server",
		Runner: sigmon.New(runner),
	}

	tsaDebugMember := grouper.Member{
		Name: "debug-server",
		Runner: http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)}

	members := []grouper.Member{
		tsaDebugMember,
		tsaServerMember,
	}

	group := grouper.NewParallel(os.Interrupt, members)
	return <-ifrit.Invoke(group).Wait()
}

func (cmd *TSACommand) Runner(args []string) (ifrit.Runner, error) {
	logger, _ := cmd.constructLogger()

	atcEndpointPicker := tsa.NewRandomATCEndpointPicker(cmd.ATCURLs)

	teamAuthorizedKeys, err := cmd.loadTeamAuthorizedKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to load team authorized keys: %s", err)
	}

	sessionAuthTeam := &sessionTeam{
		sessionTeams: make(map[string]string),
		lock:         &sync.RWMutex{},
	}

	config, err := cmd.configureSSHServer(sessionAuthTeam, cmd.AuthorizedKeys.Keys, teamAuthorizedKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to configure SSH server: %s", err)
	}

	listenAddr := fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)

	if cmd.SessionSigningKey != nil {
		tokenGenerator := tsa.NewTokenGenerator(cmd.SessionSigningKey.PrivateKey)

		server := &registrarSSHServer{
			logger:            logger,
			heartbeatInterval: cmd.HeartbeatInterval,
			cprInterval:       1 * time.Second,
			atcEndpointPicker: atcEndpointPicker,
			tokenGenerator:    tokenGenerator,
			forwardHost:       cmd.PeerIP,
			config:            config,
			httpClient:        http.DefaultClient,
			sessionTeam:       sessionAuthTeam,
		}
		return serverRunner{logger, server, listenAddr}, nil
	}
	return nil, fmt.Errorf("missing session signing key")
}

func (cmd *TSACommand) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("tsa")

	return logger, reconfigurableSink
}

func (cmd *TSACommand) loadTeamAuthorizedKeys() ([]TeamAuthKeys, error) {
	var teamKeys []TeamAuthKeys

	for i := range cmd.TeamAuthorizedKeys {
		var teamAuthorizedKeys []ssh.PublicKey

		teamAuthKeysBytes, err := ioutil.ReadFile(string(cmd.TeamAuthorizedKeys[i].Path))

		if err != nil {
			return nil, err
		}

		for {
			key, _, _, rest, err := ssh.ParseAuthorizedKey(teamAuthKeysBytes)
			if err != nil {
				break
			}

			teamAuthorizedKeys = append(teamAuthorizedKeys, key)

			teamAuthKeysBytes = rest
		}

		teamKeys = append(teamKeys, TeamAuthKeys{Team: cmd.TeamAuthorizedKeys[i].Name, AuthKeys: teamAuthorizedKeys})
	}

	return teamKeys, nil
}

func (cmd *TSACommand) configureSSHServer(sessionAuthTeam *sessionTeam, authorizedKeys []ssh.PublicKey, teamAuthorizedKeys []TeamAuthKeys) (*ssh.ServerConfig, error) {
	certChecker := &ssh.CertChecker{
		IsUserAuthority: func(key ssh.PublicKey) bool {
			return false
		},

		IsHostAuthority: func(key ssh.PublicKey, address string) bool {
			return false
		},

		UserKeyFallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			for _, k := range authorizedKeys {
				if bytes.Equal(k.Marshal(), key.Marshal()) {
					return nil, nil
				}
			}

			for _, teamKeys := range teamAuthorizedKeys {
				for _, k := range teamKeys.AuthKeys {
					if bytes.Equal(k.Marshal(), key.Marshal()) {
						sessionAuthTeam.AuthorizeTeam(string(conn.SessionID()), teamKeys.Team)
						return nil, nil
					}
				}
			}

			return nil, fmt.Errorf("unknown public key")
		},
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return certChecker.Authenticate(conn, key)
		},
	}

	signer, err := ssh.NewSignerFromKey(cmd.HostKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer from host key: %s", err)
	}

	config.AddHostKey(signer)

	return config, nil
}
