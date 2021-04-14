package tsacmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"io/ioutil"

	yaml "gopkg.in/yaml.v2"

	"os/signal"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
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

// IMPORTANT! The env tags are only added for backwards compatibility sake. Any
// new fields do NOT require the env tag
type TSAConfig struct {
	Logger flag.Lager `yaml:",inline"`

	BindIP      net.IP `yaml:"bind_ip,omitempty" env:"CONCOURSE_WORKER_GATEWAY_BIND_IP,CONCOURSE_TSA_BIND_IP"`
	PeerAddress string `yaml:"peer_address,omitempty" env:"CONCOURSE_WORKER_GATEWAY_PEER_ADDRESS,CONCOURSE_TSA_PEER_ADDRESS"`
	BindPort    uint16 `yaml:"bind_port,omitempty" env:"CONCOURSE_WORKER_GATEWAY_BIND_PORT,CONCOURSE_TSA_BIND_PORT"`

	Debug DebugConfig `yaml:"debug,omitempty"`

	HostKey                *flag.PrivateKey       `yaml:"host_key,omitempty" validate:"required" env:"CONCOURSE_WORKER_GATEWAY_HOST_KEY,CONCOURSE_TSA_HOST_KEY"`
	AuthorizedKeys         flag.AuthorizedKeys    `yaml:"authorized_keys,omitempty" env:"CONCOURSE_WORKER_GATEWAY_AUTHORIZED_KEYS,CONCOURSE_TSA_AUTHORIZED_KEYS"`
	TeamAuthorizedKeys     flag.AuthorizedKeysMap `yaml:"team_authorized_keys,omitempty" env:"CONCOURSE_WORKER_GATEWAY_TEAM_AUTHORIZED_KEYS,CONCOURSE_TSA_TEAM_AUTHORIZED_KEYS"`
	TeamAuthorizedKeysFile flag.File              `yaml:"team_authorized_keys_file,omitempty" env:"CONCOURSE_WORKER_GATEWAY_TEAM_AUTHORIZED_KEYS_FILE,CONCOURSE_TSA_TEAM_AUTHORIZED_KEYS_FILE"`

	ATCURLs flag.URLs `yaml:"atc_url,omitempty" env:"CONCOURSE_WORKER_GATEWAY_ATC_URL,CONCOURSE_TSA_ATC_URL"`

	ClientID     string   `yaml:"client_id,omitempty" env:"CONCOURSE_WORKER_GATEWAY_CLIENT_ID,CONCOURSE_TSA_CLIENT_ID"`
	ClientSecret string   `yaml:"client_secret,omitempty" env:"CONCOURSE_WORKER_GATEWAY_CLIENT_SECRET,CONCOURSE_TSA_CLIENT_SECRET"`
	TokenURL     flag.URL `yaml:"token_url,omitempty" env:"CONCOURSE_WORKER_GATEWAY_TOKEN_URL,CONCOURSE_TSA_TOKEN_URL"`
	Scopes       []string `yaml:"scope,omitempty" env:"CONCOURSE_WORKER_GATEWAY_SCOPE,CONCOURSE_TSA_SCOPE"`

	HeartbeatInterval    time.Duration `yaml:"heartbeat_interval,omitempty" env:"CONCOURSE_WORKER_GATEWAY_HEARTBEAT_INTERVAL,CONCOURSE_TSA_HEARTBEAT_INTERVAL"`
	GardenRequestTimeout time.Duration `yaml:"garden_request_timeout,omitempty" env:"CONCOURSE_WORKER_GATEWAY_GARDEN_REQUEST_TIMEOUT,CONCOURSE_TSA_GARDEN_REQUEST_TIMEOUT"`

	ClusterName    string `yaml:"cluster_name,omitempty" env:"CONCOURSE_WORKER_GATEWAY_CLUSTER_NAME,CONCOURSE_TSA_CLUSTER_NAME"`
	LogClusterName bool   `yaml:"log_cluster_name,omitempty" env:"CONCOURSE_WORKER_GATEWAY_LOG_CLUSTER_NAME,CONCOURSE_TSA_LOG_CLUSTER_NAME"`
}

type DebugConfig struct {
	BindIP   net.IP `yaml:"bind_ip,omitempty" env:"CONCOURSE_WORKER_GATEWAY_DEBUG_BIND_IP,CONCOURSE_TSA_DEBUG_BIND_IP"`
	BindPort uint16 `yaml:"bind_port,omitempty" env:"CONCOURSE_WORKER_GATEWAY_DEBUG_BIND_PORT,CONCOURSE_TSA_DEBUG_BIND_PORT"`
}

var CmdDefaults = TSAConfig{
	Logger: flag.Lager{
		LogLevel: "info",
	},

	BindIP:      net.ParseIP("0.0.0.0"),
	PeerAddress: "127.0.0.1",
	BindPort:    2222,

	Debug: DebugConfig{
		BindIP:   net.ParseIP("127.0.0.1"),
		BindPort: 2221,
	},

	ClientID: "concourse-worker",

	HeartbeatInterval:    30 * time.Second,
	GardenRequestTimeout: 5 * time.Minute,
}

type TeamAuthKeys struct {
	Team     string
	AuthKeys []ssh.PublicKey
}

type yamlTeamAuthorizedKey struct {
	Team string   `yaml:"team"`
	Keys []string `yaml:"ssh_keys,flow"`
}

func (cmd *TSAConfig) Execute(args []string) error {
	fmt.Printf("XX %+v", &cmd)
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

func (cmd *TSAConfig) Runner(args []string) (ifrit.Runner, error) {
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
	httpClient := oauth2.NewClient(ctx, tokenSource)

	server := &server{
		logger:               logger,
		heartbeatInterval:    cmd.HeartbeatInterval,
		cprInterval:          1 * time.Second,
		atcEndpointPicker:    atcEndpointPicker,
		forwardHost:          cmd.PeerAddress,
		config:               config,
		httpClient:           httpClient,
		sessionTeam:          sessionAuthTeam,
		gardenRequestTimeout: cmd.GardenRequestTimeout,
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

func (cmd *TSAConfig) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("tsa")
	if cmd.LogClusterName {
		logger = logger.WithData(lager.Data{
			"cluster": cmd.ClusterName,
		})
	}

	return logger, reconfigurableSink
}

func (cmd *TSAConfig) loadTeamAuthorizedKeys() ([]TeamAuthKeys, error) {
	var teamKeys []TeamAuthKeys

	for teamName, keys := range cmd.TeamAuthorizedKeys {
		teamKeys = append(teamKeys, TeamAuthKeys{
			Team:     teamName,
			AuthKeys: keys.Keys,
		})
	}

	// load TeamAuthorizedKeysFile
	if cmd.TeamAuthorizedKeysFile != "" {
		teamAuthorizedKeysAbs, err := filepath.Abs(cmd.TeamAuthorizedKeysFile.Path())
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path from team authorized keys file: %s", err)
		}

		logger, _ := cmd.constructLogger()
		var rawTeamAuthorizedKeys []yamlTeamAuthorizedKey

		authorizedKeysBytes, err := ioutil.ReadFile(teamAuthorizedKeysAbs)
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

func (cmd *TSAConfig) configureSSHServer(sessionAuthTeam *sessionTeam, authorizedKeys []ssh.PublicKey, teamAuthorizedKeys []TeamAuthKeys) (*ssh.ServerConfig, error) {
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

func (cmd *TSAConfig) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.Debug.BindIP, cmd.Debug.BindPort)
}

func (cmd *TSAConfig) parseAuthorizedKeys(keys string) ([]ssh.PublicKey, error) {
	authorizedKeysBytes, err := ioutil.ReadFile(keys)
	if err != nil {
		return nil, fmt.Errorf("failed to read authorized keys: %s", err)
	}

	var authorizedKeys []ssh.PublicKey

	for {
		key, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			// there's no good error to check for here
			break
		}

		authorizedKeys = append(authorizedKeys, key)

		authorizedKeysBytes = rest
	}

	return authorizedKeys, nil
}
