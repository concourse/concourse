package tsacmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"io/ioutil"

	yaml "gopkg.in/yaml.v2"

	"os/signal"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/concourse/tsa"
	"github.com/concourse/flag"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type TSACommand struct {
	Logger flag.Lager

	BindIP      flag.IP `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for SSH."`
	PeerAddress string  `long:"peer-address" default:"127.0.0.1" description:"Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses."`
	BindPort    uint16  `long:"bind-port" default:"2222"    description:"Port on which to listen for SSH."`

	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16  `long:"debug-bind-port" default:"2221"      description:"Port on which to listen for the pprof debugger endpoints."`

	HostKey                *flag.PrivateKey               `long:"host-key"        required:"true" description:"Path to private key to use for the SSH server."`
	AuthorizedKeys         flag.AuthorizedKeys            `long:"authorized-keys" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`
	TeamAuthorizedKeys     map[string]flag.AuthorizedKeys `long:"team-authorized-keys" value-name:"NAME:PATH" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`
	TeamAuthorizedKeysFile flag.File                      `long:"team-authorized-keys-file" description:"Path to file containing a YAML array of teams and their authorized SSH keys, e.g. [{team:foo,ssh_keys:[key1,key2]}]."`

	ATCURLs []flag.URL `long:"atc-url" required:"true" description:"ATC API endpoints to which workers will be registered."`

	ClientID     string   `long:"client-id" default:"concourse-worker" description:"Client used to fetch a token from the auth server. NOTE: if you change this value you will also need to change the --system-claim-value flag so the atc knows to allow requests from this client."`
	ClientSecret string   `long:"client-secret" required:"true" description:"Client used to fetch a token from the auth server"`
	TokenURL     flag.URL `long:"token-url" required:"true" description:"Token endpoint of the auth server"`
	Scopes       []string `long:"scope" description:"Scopes to request from the auth server"`

	HeartbeatInterval time.Duration `long:"heartbeat-interval" default:"30s" description:"interval on which to heartbeat workers to the ATC"`

	ClusterName    string `long:"cluster-name" description:"A name for this Concourse cluster, to be displayed on the dashboard page."`
	LogClusterName bool   `long:"log-cluster-name" description:"Log cluster name."`
}

type TeamAuthKeys struct {
	Team     string
	AuthKeys []ssh.PublicKey
}

type yamlTeamAuthorizedKey struct {
	Team string   `yaml:"team"`
	Keys []string `yaml:"ssh_keys,flow"`
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

	if len(cmd.AuthorizedKeys.Keys)+len(cmd.TeamAuthorizedKeys) == 0 {
		logger.Info("starting-tsa-without-authorized-keys")
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

	authConfig := clientcredentials.Config{
		ClientID:     cmd.ClientID,
		ClientSecret: cmd.ClientSecret,
		TokenURL:     cmd.TokenURL.URL.String(),
		Scopes:       cmd.Scopes,
	}

	ctx := context.Background()

	tokenSource := authConfig.TokenSource(ctx)
	idTokenSource := token.NewTokenSource(tokenSource)
	httpClient := oauth2.NewClient(ctx, idTokenSource)

	server := &server{
		logger:            logger,
		heartbeatInterval: cmd.HeartbeatInterval,
		cprInterval:       1 * time.Second,
		atcEndpointPicker: atcEndpointPicker,
		forwardHost:       cmd.PeerAddress,
		config:            config,
		httpClient:        httpClient,
		sessionTeam:       sessionAuthTeam,
	}
	// Starts a goroutine whose purpose is to listen to the
	// SIGHUP syscall and reload configuration upon receiving the signal.
	// For now it only reloads the TSACommand.AuthorizedKeys but
	// other configuration can potentially be added.
	go func() {
		reloadWorkerKeys := make(chan os.Signal, 1)
		defer close(reloadWorkerKeys)
		signal.Notify(reloadWorkerKeys, syscall.SIGHUP)
		for {

			// Block until a signal is received.
			<-reloadWorkerKeys

			logger.Info("reloading-config")

			err := cmd.AuthorizedKeys.Reload()
			if err != nil {
				logger.Error("failed to reload authorized keys file : %s", err)
				continue
			}

			teamAuthorizedKeys, err = cmd.loadTeamAuthorizedKeys()
			if err != nil {
				logger.Error("failed to load team authorized keys : %s", err)
				continue
			}

			// Reconfigure the SSH server with the new keys
			config, err := cmd.configureSSHServer(sessionAuthTeam, cmd.AuthorizedKeys.Keys, teamAuthorizedKeys)
			if err != nil {
				logger.Error("failed to configure SSH server: %s", err)
				continue
			}

			server.config = config
		}
	}()

	return serverRunner{logger, server, listenAddr}, nil
}

func (cmd *TSACommand) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("tsa")
	if cmd.LogClusterName {
		logger = logger.WithData(lager.Data{
			"cluster": cmd.ClusterName,
		})
	}

	return logger, reconfigurableSink
}

func (cmd *TSACommand) loadTeamAuthorizedKeys() ([]TeamAuthKeys, error) {
	var teamKeys []TeamAuthKeys

	for teamName, keys := range cmd.TeamAuthorizedKeys {
		teamKeys = append(teamKeys, TeamAuthKeys{
			Team:     teamName,
			AuthKeys: keys.Keys,
		})
	}

	// load TeamAuthorizedKeysFile
	if cmd.TeamAuthorizedKeysFile != "" {
		logger, _ := cmd.constructLogger()
		var rawTeamAuthorizedKeys []yamlTeamAuthorizedKey

		authorizedKeysBytes, err := ioutil.ReadFile(cmd.TeamAuthorizedKeysFile.Path())
		if err != nil {
			return nil, fmt.Errorf("failed to read yaml authorized keys file: %s", err)
		}
		err = yaml.Unmarshal([]byte(authorizedKeysBytes), &rawTeamAuthorizedKeys)
		if err != nil {
			return nil, fmt.Errorf("failed to parse yaml authorized keys file: %s", err)
		}

		for _, t := range rawTeamAuthorizedKeys {
			var teamAuthorizedKeys []ssh.PublicKey
			for _, k := range t.Keys {
				key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k))
				if err != nil {
					logger.Error("load-team-authorized-keys-parse", fmt.Errorf("Invalid format, ignoring (%s): %s", k, err.Error()))
					continue
				}
				logger.Info("load-team-authorized-keys-loaded", lager.Data{"team": t.Team, "key": k})
				teamAuthorizedKeys = append(teamAuthorizedKeys, key)
			}
			teamKeys = append(teamKeys, TeamAuthKeys{Team: t.Team, AuthKeys: teamAuthorizedKeys})
		}
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
		Config: atc.DefaultSSHConfig(),
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

func (cmd *TSACommand) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.DebugBindIP, cmd.DebugBindPort)
}
