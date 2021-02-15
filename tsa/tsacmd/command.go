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
	"github.com/concourse/concourse/flag"
	"github.com/concourse/concourse/tsa"
	cflag "github.com/concourse/flag"
	"github.com/dgrijalva/jwt-go"
	"github.com/spf13/cobra"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type TSACommand struct {
	Logger cflag.Lager

	BindIP      flag.IP `yaml:"bind_ip"`
	PeerAddress string  `yaml:"peer_address"`
	BindPort    uint16  `yaml:"bind_port"`

	Debug struct {
		BindIP   flag.IP `yaml:"bind_ip"`
		BindPort uint16  `yaml:"bind_port"`
	} `yaml:"debug"`

	HostKey                *flag.PrivateKey  `yaml:"host_key" validate:"required"`
	AuthorizedKeys         string            `yaml:"authorized_keys"`
	TeamAuthorizedKeys     map[string]string `yaml:"team_authorized_keys"`
	TeamAuthorizedKeysFile string            `yaml:"team_authorized_keys_file" validate:"file"`

	ATCURLs []string `yaml:"atc_url" validate:"dive,url"`

	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	TokenURL     string   `yaml:"token_url" validate:"url"`
	Scopes       []string `yaml:"scope"`

	HeartbeatInterval    time.Duration `yaml:"heartbeat_interval"`
	GardenRequestTimeout time.Duration `long:"garden-request-timeout" default:"5m" description:"How long to wait for requests to Garden to complete. 0 means no timeout."`

	ClusterName    string `yaml:"cluster_name"`
	LogClusterName bool   `yaml:"log_cluster_name"`
}

func InitializeFlagsDEPRECATED(c *cobra.Command, flags *TSACommand) {
	c.Flags().Var(&flags.BindIP, "tsa-bind-ip", "IP address on which to listen for SSH.")
	c.Flags().Uint16Var(&flags.BindPort, "tsa-bind-port", 2222, "Port on which to listen for SSH.")
	c.Flags().StringVar(&flags.PeerAddress, "tsa-peer-address", "127.0.0.1", "Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses.")

	c.Flags().Var(&flags.Debug.BindIP, "tsa-debug-bind-ip", "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Debug.BindPort, "tsa-debug-bind-port", 2221, "Port on which to listen for the pprof debugger endpoints.")

	c.Flags().Var(flags.HostKey, "tsa-host-key", "Path to private key to use for the SSH server.")
	c.Flags().StringVar(&flags.AuthorizedKeys, "tsa-authorized-keys", "", "Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line).")
	c.Flags().StringToStringVar(&flags.TeamAuthorizedKeys, "tsa-team-authorized-keys", nil, "Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line).")
	c.Flags().StringVar(&flags.TeamAuthorizedKeysFile, "tsa-team-authorized-keys-file", "", "Path to file containing a YAML array of teams and their authorized SSH keys, e.g. [{team:foo,ssh_keys:[key1,key2]}].")

	c.Flags().StringArrayVar(&flags.ATCURLs, "tsa-atc-url", nil, "ATC API endpoints to which workers will be registered.")

	c.Flags().StringVar(&flags.ClientID, "tsa-client-id", "concourse-worker", "Client used to fetch a token from the auth server. NOTE: if you change this value you will also need to change the --system-claim-value flag so the atc knows to allow requests from this client.")
	c.Flags().StringVar(&flags.ClientSecret, "tsa-client-secret", "", "Client used to fetch a token from the auth server")
	c.Flags().StringVar(&flags.TokenURL, "tsa-token-url", "", "Token endpoint of the auth server")
	c.Flags().StringArrayVar(&flags.Scopes, "tsa-scope", nil, "Scopes to request from the auth server")

	c.Flags().DurationVar(&flags.HeartbeatInterval, "tsa-heartbeat-interval", 30*time.Second, "interval on which to heartbeat workers to the ATC")

	c.Flags().StringVar(&flags.ClusterName, "tsa-cluster-name", "", "A name for this Concourse cluster, to be displayed on the dashboard page.")
	c.Flags().BoolVar(&flags.LogClusterName, "tsa-log-cluster-name", false, "Log cluster name.")
}

func SetDefaults(cmd *TSACommand) {
	cmd.BindIP = flag.IP{net.ParseIP("0.0.0.0")}
	cmd.PeerAddress = "127.0.0.1"
	cmd.BindPort = 2222

	cmd.Debug.BindIP = flag.IP{net.ParseIP("127.0.0.1")}
	cmd.Debug.BindPort = 2221

	cmd.ClientID = "concourse-worker"

	cmd.HeartbeatInterval = 30 * time.Second
}

// type TSACommand struct {
// 	Logger flag.Lager

// 	BindIP      flag.IP `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for SSH."`
// 	PeerAddress string  `long:"peer-address" default:"127.0.0.1" description:"Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses."`
// 	BindPort    uint16  `long:"bind-port" default:"2222"    description:"Port on which to listen for SSH."`

// 	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
// 	DebugBindPort uint16  `long:"debug-bind-port" default:"2221"      description:"Port on which to listen for the pprof debugger endpoints."`

// 	HostKey                *flag.PrivateKey               `long:"host-key"        required:"true" description:"Path to private key to use for the SSH server."`
// 	AuthorizedKeys         flag.AuthorizedKeys            `long:"authorized-keys" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`
// 	TeamAuthorizedKeys     map[string]flag.AuthorizedKeys `long:"team-authorized-keys" value-name:"NAME:PATH" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`
// TeamAuthorizedKeysFile flag.File                      `long:"team-authorized-keys-file" description:"Path to file containing a YAML array of teams and their authorized SSH keys, e.g. [{team:foo,ssh_keys:[key1,key2]}]."`

// 	ATCURLs []flag.URL `long:"atc-url" required:"true" description:"ATC API endpoints to which workers will be registered."`

// 	ClientID     string   `long:"client-id" default:"concourse-worker" description:"Client used to fetch a token from the auth server. NOTE: if you change this value you will also need to change the --system-claim-value flag so the atc knows to allow requests from this client."`
// 	ClientSecret string   `long:"client-secret" required:"true" description:"Client used to fetch a token from the auth server"`
// 	TokenURL     flag.URL `long:"token-url" required:"true" description:"Token endpoint of the auth server"`
// 	Scopes       []string `long:"scope" description:"Scopes to request from the auth server"`

// 	HeartbeatInterval time.Duration `long:"heartbeat-interval" default:"30s" description:"interval on which to heartbeat workers to the ATC"`

// 	ClusterName    string `long:"cluster-name" description:"A name for this Concourse cluster, to be displayed on the dashboard page."`
// 	LogClusterName bool   `long:"log-cluster-name" description:"Log cluster name."`
// }

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
		teamAuthorizedKeysAbs, err := filepath.Abs(cmd.TeamAuthorizedKeysFile)
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

	rsaKeyBlob, err := ioutil.ReadFile(cmd.HostKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file (%s): %s", cmd.HostKey, err)
	}

	hostKey, err := jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer from host key: %s", err)
	}

	config.AddHostKey(signer)

	return config, nil
}

func (cmd *TSACommand) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.Debug.BindIP, cmd.Debug.BindPort)
}

func (cmd *TSACommand) parseAuthorizedKeys(keys string) ([]ssh.PublicKey, error) {
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
