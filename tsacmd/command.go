package tsacmd

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/tsa"
	"github.com/dgrijalva/jwt-go"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"
	"github.com/xoebus/zest"
)

type TSACommand struct {
	BindIP   IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for SSH."`
	BindPort uint16 `long:"bind-port" default:"2222"    description:"Port on which to listen for SSH."`

	PeerIP string `long:"peer-ip" required:"true" description:"IP address of this TSA, reachable by the ATCs. Used for forwarded worker addresses."`

	HostKeyPath            FileFlag        `long:"host-key"        required:"true" description:"Path to private key to use for the SSH server."`
	AuthorizedKeysPath     FileFlag        `long:"authorized-keys" required:"true" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`
	TeamAuthorizedKeysPath []InputPairFlag `long:"team-authorized-keys" value-name:"NAME=PATH" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`

	ATCURL                URLFlag  `long:"atc-url" required:"true" description:"ATC API endpoint to which workers will be registered."`
	SessionSigningKeyPath FileFlag `long:"session-signing-key" required:"true" description:"Path to private key to use when signing tokens in reqests to the ATC during registration."`

	HeartbeatInterval time.Duration `long:"heartbeat-interval" default:"30s" description:"interval on which to heartbeat workers to the ATC"`

	Metrics struct {
		YellerAPIKey      string `long:"yeller-api-key"     description:"Yeller API key. If specified, all errors logged will be emitted."`
		YellerEnvironment string `long:"yeller-environment" description:"Environment to tag on all Yeller events emitted."`
	} `group:"Metrics & Diagnostics"`
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

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *TSACommand) Runner(args []string) (ifrit.Runner, error) {
	logger := lager.NewLogger("tsa")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	if cmd.Metrics.YellerAPIKey != "" {
		yellerSink := zest.NewYellerSink(cmd.Metrics.YellerAPIKey, cmd.Metrics.YellerEnvironment)
		logger.RegisterSink(yellerSink)
	}

	atcEndpoint := rata.NewRequestGenerator(cmd.ATCURL.String(), atc.Routes)

	authorizedKeys, err := cmd.loadAuthorizedKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to load authorized keys: %s", err)
	}

	teamAuthorizedKeys, err := cmd.loadTeamAuthorizedKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to load team authorized keys: %s", err)
	}

	sessionSigningKey, err := cmd.loadSessionSigningKey()
	if err != nil {
		return nil, fmt.Errorf("failed to load session signing key: %s", err)
	}

	sessionAuthTeam := make(sessionTeam)

	config, err := cmd.configureSSHServer(sessionAuthTeam, authorizedKeys, teamAuthorizedKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to configure SSH server: %s", err)
	}

	listenAddr := fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)

	tokenGenerator := tsa.NewTokenGenerator(sessionSigningKey)

	server := &registrarSSHServer{
		logger:            logger,
		heartbeatInterval: cmd.HeartbeatInterval,
		cprInterval:       1 * time.Second,
		atcEndpoint:       atcEndpoint,
		tokenGenerator:    tokenGenerator,
		forwardHost:       cmd.PeerIP,
		config:            config,
		httpClient:        http.DefaultClient,
		sessionTeam:       sessionAuthTeam,
	}

	return serverRunner{logger, server, listenAddr}, nil
}

func (cmd *TSACommand) loadAuthorizedKeys() ([]ssh.PublicKey, error) {
	authorizedKeysBytes, err := ioutil.ReadFile(string(cmd.AuthorizedKeysPath))
	if err != nil {
		return nil, err
	}

	var authorizedKeys []ssh.PublicKey

	for {
		key, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			break
		}

		authorizedKeys = append(authorizedKeys, key)

		authorizedKeysBytes = rest
	}

	return authorizedKeys, nil
}

func (cmd *TSACommand) loadTeamAuthorizedKeys() ([]TeamAuthKeys, error) {
	var teamKeys []TeamAuthKeys

	for i := range cmd.TeamAuthorizedKeysPath {
		var teamAuthorizedKeys []ssh.PublicKey

		teamAuthKeysBytes, err := ioutil.ReadFile(string(cmd.TeamAuthorizedKeysPath[i].Path))

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

		teamKeys = append(teamKeys, TeamAuthKeys{Team: cmd.TeamAuthorizedKeysPath[i].Name, AuthKeys: teamAuthorizedKeys})
	}

	return teamKeys, nil
}

func (cmd *TSACommand) loadSessionSigningKey() (*rsa.PrivateKey, error) {
	rsaKeyBlob, err := ioutil.ReadFile(string(cmd.SessionSigningKeyPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read session signing key file: %s", err)
	}

	signingKey, err := jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
	if err != nil {
		return nil, fmt.Errorf("failed to parse session signing key as RSA: %s", err)
	}

	return signingKey, nil
}

func (cmd *TSACommand) configureSSHServer(sessionAuthTeam sessionTeam, authorizedKeys []ssh.PublicKey, teamAuthorizedKeys []TeamAuthKeys) (*ssh.ServerConfig, error) {
	certChecker := &ssh.CertChecker{
		IsAuthority: func(key ssh.PublicKey) bool {
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
						sessionAuthTeam[string(conn.SessionID())] = teamKeys.Team
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

	privateBytes, err := ioutil.ReadFile(string(cmd.HostKeyPath))
	if err != nil {
		return nil, err
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, err
	}

	config.AddHostKey(private)

	return config, nil
}
