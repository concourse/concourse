package atccmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/buildserver"
	"github.com/concourse/concourse/atc/api/containerserver"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/api/policychecker"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/component"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/noop"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/atc/syslog"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/image"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/skymarshal/dexserver"
	"github.com/concourse/concourse/skymarshal/legacyserver"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/skymarshal/skyserver"
	"github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/web"
	"github.com/concourse/flag"
	"github.com/concourse/retryhttp"

	"github.com/cppforlife/go-semi-semantic/version"
	multierror "github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	gocache "github.com/patrickmn/go-cache"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"

	// dynamically registered metric emitters
	_ "github.com/concourse/concourse/atc/metric/emitter"

	// dynamically registered policy checkers
	_ "github.com/concourse/concourse/atc/policy/opa"

	// dynamically registered credential managers
	_ "github.com/concourse/concourse/atc/creds/conjur"
	_ "github.com/concourse/concourse/atc/creds/credhub"
	_ "github.com/concourse/concourse/atc/creds/dummy"
	_ "github.com/concourse/concourse/atc/creds/kubernetes"
	_ "github.com/concourse/concourse/atc/creds/secretsmanager"
	_ "github.com/concourse/concourse/atc/creds/ssm"
	_ "github.com/concourse/concourse/atc/creds/vault"
)

const algorithmLimitRows = 100

var schedulerCache = gocache.New(10*time.Second, 10*time.Second)

var defaultDriverName = "postgres"
var retryingDriverName = "too-many-connections-retrying"

var flyClientID = "fly"
var flyClientSecret = "Zmx5"

var workerAvailabilityPollingInterval = 5 * time.Second
var workerStatusPublishInterval = 1 * time.Minute
var BatcherInterval = 15 * time.Second

type ATCCommand struct {
	RunCommand RunCommand `command:"run"`
	Migration  Migration  `command:"migrate"`
}

type RunCommand struct {
	Logger flag.Lager

	varSourcePool creds.VarSourcePool

	BindIP   flag.IP `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for web traffic."`
	BindPort uint16  `long:"bind-port" default:"8080"    description:"Port on which to listen for HTTP traffic."`

	TLSBindPort uint16    `long:"tls-bind-port" description:"Port on which to listen for HTTPS traffic."`
	TLSCert     flag.File `long:"tls-cert"      description:"File containing an SSL certificate."`
	TLSKey      flag.File `long:"tls-key"       description:"File containing an RSA private key, used to encrypt HTTPS traffic."`

	LetsEncrypt struct {
		Enable  bool     `long:"enable-lets-encrypt"   description:"Automatically configure TLS certificates via Let's Encrypt/ACME."`
		ACMEURL flag.URL `long:"lets-encrypt-acme-url" description:"URL of the ACME CA directory endpoint." default:"https://acme-v02.api.letsencrypt.org/directory"`
	} `group:"Let's Encrypt Configuration"`

	ExternalURL flag.URL `long:"external-url" description:"URL used to reach any ATC from the outside world."`

	Postgres flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`

	ConcurrentRequestLimits   map[wrappa.LimitedRoute]int `long:"concurrent-request-limit" description:"Limit the number of concurrent requests to an API endpoint (Example: ListAllJobs:5)"`
	APIMaxOpenConnections     int                         `long:"api-max-conns" description:"The maximum number of open connections for the api connection pool." default:"10"`
	BackendMaxOpenConnections int                         `long:"backend-max-conns" description:"The maximum number of open connections for the backend connection pool." default:"50"`

	CredentialManagement creds.CredentialManagementConfig `group:"Credential Management"`
	CredentialManagers   creds.Managers

	EncryptionKey    flag.Cipher `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	OldEncryptionKey flag.Cipher `long:"old-encryption-key" description:"Encryption key previously used for encrypting sensitive information. If provided without a new key, data is encrypted. If provided with a new key, data is re-encrypted."`

	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16  `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	InterceptIdleTimeout time.Duration `long:"intercept-idle-timeout" default:"0m" description:"Length of time for a intercepted session to be idle before terminating."`

	EnableGlobalResources bool `long:"enable-global-resources" description:"Enable equivalent resources across pipelines and teams to share a single version history."`

	ComponentRunnerInterval time.Duration `long:"component-runner-interval" default:"10s" description:"Interval on which runners are kicked off for builds, locks, scans, and checks"`

	LidarScannerInterval time.Duration `long:"lidar-scanner-interval" default:"10s" description:"Interval on which the resource scanner will run to see if new checks need to be scheduled"`
	LidarCheckerInterval time.Duration `long:"lidar-checker-interval" default:"10s" description:"Interval on which the resource checker runs any scheduled checks"`

	GlobalResourceCheckTimeout          time.Duration `long:"global-resource-check-timeout" default:"1h" description:"Time limit on checking for new versions of resources."`
	ResourceCheckingInterval            time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`
	ResourceWithWebhookCheckingInterval time.Duration `long:"resource-with-webhook-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources that has webhook defined."`
	MaxChecksPerSecond                  int           `long:"max-checks-per-second" description:"Maximum number of checks that can be started per second. If not specified, this will be calculated as (# of resources)/(resource checking interval). -1 value will remove this maximum limit of checks per second."`

	ContainerPlacementStrategy        string        `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" description:"Method by which a worker is selected during container placement."`
	MaxActiveTasksPerWorker           int           `long:"max-active-tasks-per-worker" default:"0" description:"Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit."`
	BaggageclaimResponseHeaderTimeout time.Duration `long:"baggageclaim-response-header-timeout" default:"1m" description:"How long to wait for Baggageclaim to send the response header."`
	StreamingArtifactsCompression     string        `long:"streaming-artifacts-compression" default:"gzip" choice:"gzip" choice:"zstd" description:"Compression algorithm for internal streaming."`

	GardenRequestTimeout time.Duration `long:"garden-request-timeout" default:"5m" description:"How long to wait for requests to Garden to complete. 0 means no timeout."`

	CLIArtifactsDir flag.Dir `long:"cli-artifacts-dir" description:"Directory containing downloadable CLI binaries."`

	Metrics struct {
		HostName            string            `long:"metrics-host-name" description:"Host string to attach to emitted metrics."`
		Attributes          map[string]string `long:"metrics-attribute" description:"A key-value attribute to attach to emitted metrics. Can be specified multiple times." value-name:"NAME:VALUE"`
		BufferSize          uint32            `long:"metrics-buffer-size" default:"1000" description:"The size of the buffer used in emitting event metrics."`
		CaptureErrorMetrics bool              `long:"capture-error-metrics" description:"Enable capturing of error log metrics"`
	} `group:"Metrics & Diagnostics"`

	Tracing tracing.Config `group:"Tracing" namespace:"tracing"`

	PolicyCheckers struct {
		Filter policy.Filter
	} `group:"Policy Checking"`

	Server struct {
		XFrameOptions string `long:"x-frame-options" default:"deny" description:"The value to set for X-Frame-Options."`
		ClusterName   string `long:"cluster-name" description:"A name for this Concourse cluster, to be displayed on the dashboard page."`
		ClientID      string `long:"client-id" default:"concourse-web" description:"Client ID to use for login flow"`
		ClientSecret  string `long:"client-secret" required:"true" description:"Client secret to use for login flow"`
	} `group:"Web Server"`

	LogDBQueries   bool `long:"log-db-queries" description:"Log database queries."`
	LogClusterName bool `long:"log-cluster-name" description:"Log cluster name."`

	GC struct {
		Interval time.Duration `long:"interval" default:"30s" description:"Interval on which to perform garbage collection."`

		OneOffBuildGracePeriod time.Duration `long:"one-off-grace-period" default:"5m" description:"Period after which one-off build containers will be garbage-collected."`
		MissingGracePeriod     time.Duration `long:"missing-grace-period" default:"5m" description:"Period after which to reap containers and volumes that were created but went missing from the worker."`
		HijackGracePeriod      time.Duration `long:"hijack-grace-period" default:"5m" description:"Period after which hijacked containers will be garbage collected"`
		FailedGracePeriod      time.Duration `long:"failed-grace-period" default:"120h" description:"Period after which failed containers will be garbage collected"`
		CheckRecyclePeriod     time.Duration `long:"check-recycle-period" default:"1m" description:"Period after which to reap checks that are completed."`
	} `group:"Garbage Collection" namespace:"gc"`

	BuildTrackerInterval time.Duration `long:"build-tracker-interval" default:"10s" description:"Interval on which to run build tracking."`

	TelemetryOptIn bool `long:"telemetry-opt-in" hidden:"true" description:"Enable anonymous concourse version reporting."`

	DefaultBuildLogsToRetain uint64 `long:"default-build-logs-to-retain" description:"Default build logs to retain, 0 means all"`
	MaxBuildLogsToRetain     uint64 `long:"max-build-logs-to-retain" description:"Maximum build logs to retain, 0 means not specified. Will override values configured in jobs"`

	DefaultDaysToRetainBuildLogs uint64 `long:"default-days-to-retain-build-logs" description:"Default days to retain build logs. 0 means unlimited"`
	MaxDaysToRetainBuildLogs     uint64 `long:"max-days-to-retain-build-logs" description:"Maximum days to retain build logs, 0 means not specified. Will override values configured in jobs"`

	JobSchedulingMaxInFlight uint64 `long:"job-scheduling-max-in-flight" default:"32" description:"Maximum number of jobs to be scheduling at the same time"`

	DefaultCpuLimit    *int    `long:"default-task-cpu-limit" description:"Default max number of cpu shares per task, 0 means unlimited"`
	DefaultMemoryLimit *string `long:"default-task-memory-limit" description:"Default maximum memory per task, 0 means unlimited"`

	Auditor struct {
		EnableBuildAuditLog     bool `long:"enable-build-auditing" description:"Enable auditing for all api requests connected to builds."`
		EnableContainerAuditLog bool `long:"enable-container-auditing" description:"Enable auditing for all api requests connected to containers."`
		EnableJobAuditLog       bool `long:"enable-job-auditing" description:"Enable auditing for all api requests connected to jobs."`
		EnablePipelineAuditLog  bool `long:"enable-pipeline-auditing" description:"Enable auditing for all api requests connected to pipelines."`
		EnableResourceAuditLog  bool `long:"enable-resource-auditing" description:"Enable auditing for all api requests connected to resources."`
		EnableSystemAuditLog    bool `long:"enable-system-auditing" description:"Enable auditing for all api requests connected to system transactions."`
		EnableTeamAuditLog      bool `long:"enable-team-auditing" description:"Enable auditing for all api requests connected to teams."`
		EnableWorkerAuditLog    bool `long:"enable-worker-auditing" description:"Enable auditing for all api requests connected to workers."`
		EnableVolumeAuditLog    bool `long:"enable-volume-auditing" description:"Enable auditing for all api requests connected to volumes."`
	}

	Syslog struct {
		Hostname      string        `long:"syslog-hostname" description:"Client hostname with which the build logs will be sent to the syslog server." default:"atc-syslog-drainer"`
		Address       string        `long:"syslog-address" description:"Remote syslog server address with port (Example: 0.0.0.0:514)."`
		Transport     string        `long:"syslog-transport" description:"Transport protocol for syslog messages (Currently supporting tcp, udp & tls)."`
		DrainInterval time.Duration `long:"syslog-drain-interval" description:"Interval over which checking is done for new build logs to send to syslog server (duration measurement units are s/m/h; eg. 30s/30m/1h)" default:"30s"`
		CACerts       []string      `long:"syslog-ca-cert"              description:"Paths to PEM-encoded CA cert files to use to verify the Syslog server SSL cert."`
	} ` group:"Syslog Drainer Configuration"`

	Auth struct {
		AuthFlags     skycmd.AuthFlags
		MainTeamFlags skycmd.AuthTeamFlags `group:"Authentication (Main Team)" namespace:"main-team"`
	} `group:"Authentication"`

	EnableRedactSecrets bool `long:"enable-redact-secrets" description:"Enable redacting secrets in build logs."`

	ConfigRBAC flag.File `long:"config-rbac" description:"Customize RBAC role-action mapping."`

	SystemClaimKey        string   `long:"system-claim-key" default:"aud" description:"The token claim key to use when matching system-claim-values"`
	SystemClaimValues     []string `long:"system-claim-value" default:"concourse-worker" description:"Configure which token requests should be considered 'system' requests."`
	EnableArchivePipeline bool     `long:"enable-archive-pipeline" description:"Enable /api/v1/teams/{team}/pipelines/{pipeline}/archive endpoint."`

	EnableBuildRerunWhenWorkerDisappears bool `long:"enable-rerun-when-worker-disappears" description:"Enable automatically build rerun when worker disappears"`
}

type Migration struct {
	Postgres           flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`
	EncryptionKey      flag.Cipher         `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	CurrentDBVersion   bool                `long:"current-db-version" description:"Print the current database version and exit"`
	SupportedDBVersion bool                `long:"supported-db-version" description:"Print the max supported database version and exit"`
	MigrateDBToVersion int                 `long:"migrate-db-to-version" description:"Migrate to the specified database version and exit"`
}

func (m *Migration) Execute(args []string) error {
	if m.CurrentDBVersion {
		return m.currentDBVersion()
	}
	if m.SupportedDBVersion {
		return m.supportedDBVersion()
	}
	if m.MigrateDBToVersion > 0 {
		return m.migrateDBToVersion()
	}
	return errors.New("must specify one of `--current-db-version`, `--supported-db-version`, or `--migrate-db-to-version`")

}

func (cmd *Migration) currentDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		encryption.NewNoEncryption(),
	)

	version, err := helper.CurrentVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *Migration) supportedDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		encryption.NewNoEncryption(),
	)

	version, err := helper.SupportedVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *Migration) migrateDBToVersion() error {
	version := cmd.MigrateDBToVersion

	var newKey *encryption.Key
	if cmd.EncryptionKey.AEAD != nil {
		newKey = encryption.NewKey(cmd.EncryptionKey.AEAD)
	}

	var strategy encryption.Strategy
	if newKey != nil {
		strategy = newKey
	} else {
		strategy = encryption.NewNoEncryption()
	}

	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		strategy,
	)

	err := helper.MigrateToVersion(version)
	if err != nil {
		return fmt.Errorf("Could not migrate to version: %d Reason: %s", version, err.Error())
	}

	fmt.Println("Successfully migrated to version:", version)
	return nil
}

func (cmd *ATCCommand) WireDynamicFlags(commandFlags *flags.Command) {
	cmd.RunCommand.WireDynamicFlags(commandFlags)
}

func (cmd *RunCommand) WireDynamicFlags(commandFlags *flags.Command) {
	var (
		metricsGroup      *flags.Group
		policyChecksGroup *flags.Group
		credsGroup        *flags.Group
		authGroup         *flags.Group
	)

	groups := commandFlags.Groups()
	for i := 0; i < len(groups); i++ {
		group := groups[i]

		if credsGroup == nil && group.ShortDescription == "Credential Management" {
			credsGroup = group
		}

		if metricsGroup == nil && group.ShortDescription == "Metrics & Diagnostics" {
			metricsGroup = group
		}

		if policyChecksGroup == nil && group.ShortDescription == "Policy Checking" {
			policyChecksGroup = group
		}

		if authGroup == nil && group.ShortDescription == "Authentication" {
			authGroup = group
		}

		if metricsGroup != nil && credsGroup != nil && authGroup != nil && policyChecksGroup != nil {
			break
		}

		groups = append(groups, group.Groups()...)
	}

	if metricsGroup == nil {
		panic("could not find Metrics & Diagnostics group for registering emitters")
	}

	if policyChecksGroup == nil {
		panic("could not find Policy Checking group for registering policy checkers")
	}

	if credsGroup == nil {
		panic("could not find Credential Management group for registering managers")
	}

	if authGroup == nil {
		panic("could not find Authentication group for registering connectors")
	}

	managerConfigs := make(creds.Managers)
	for name, p := range creds.ManagerFactories() {
		managerConfigs[name] = p.AddConfig(credsGroup)
	}
	cmd.CredentialManagers = managerConfigs

	metric.WireEmitters(metricsGroup)

	policy.WireCheckers(policyChecksGroup)

	skycmd.WireConnectors(authGroup)
	skycmd.WireTeamConnectors(authGroup.Find("Authentication (Main Team)"))
}

func (cmd *RunCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *RunCommand) Runner(positionalArguments []string) (ifrit.Runner, error) {
	if cmd.ExternalURL.URL == nil {
		cmd.ExternalURL = cmd.DefaultURL()
	}

	if len(positionalArguments) != 0 {
		return nil, fmt.Errorf("unexpected positional arguments: %v", positionalArguments)
	}

	err := cmd.validate()
	if err != nil {
		return nil, err
	}

	logger, reconfigurableSink := cmd.Logger.Logger("atc")
	if cmd.LogClusterName {
		logger = logger.WithData(lager.Data{
			"cluster": cmd.Server.ClusterName,
		})
	}

	commandSession := logger.Session("cmd")
	startTime := time.Now()

	commandSession.Info("start")
	defer commandSession.Info("finish", lager.Data{
		"duration": time.Now().Sub(startTime),
	})

	atc.EnableGlobalResources = cmd.EnableGlobalResources

	//FIXME: These only need to run once for the entire binary. At the moment,
	//they rely on state of the command.
	db.SetupConnectionRetryingDriver(
		"postgres",
		cmd.Postgres.ConnectionString(),
		retryingDriverName,
	)

	// Register the sink that collects error metrics
	if cmd.Metrics.CaptureErrorMetrics {
		errorSinkCollector := metric.NewErrorSinkCollector(logger)
		logger.RegisterSink(&errorSinkCollector)
	}

	err = cmd.Tracing.Prepare()
	if err != nil {
		return nil, err
	}

	http.HandleFunc("/debug/connections", func(w http.ResponseWriter, r *http.Request) {
		for _, stack := range db.GlobalConnectionTracker.Current() {
			fmt.Fprintln(w, stack)
		}
	})

	if err := cmd.configureMetrics(logger); err != nil {
		return nil, err
	}

	lockConn, err := cmd.constructLockConn(retryingDriverName)
	if err != nil {
		return nil, err
	}

	lockFactory := lock.NewLockFactory(lockConn, metric.LogLockAcquired, metric.LogLockReleased)

	apiConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.APIMaxOpenConnections, "api", lockFactory)
	if err != nil {
		return nil, err
	}

	backendConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.BackendMaxOpenConnections, "backend", lockFactory)
	if err != nil {
		return nil, err
	}

	gcConn, err := cmd.constructDBConn(retryingDriverName, logger, 5, "gc", lockFactory)
	if err != nil {
		return nil, err
	}

	storage, err := storage.NewPostgresStorage(logger, cmd.Postgres)
	if err != nil {
		return nil, err
	}

	secretManager, err := cmd.secretManager(logger)
	if err != nil {
		return nil, err
	}

	cmd.varSourcePool = creds.NewVarSourcePool(
		logger.Session("var-source-pool"),
		5*time.Minute,
		1*time.Minute,
		clock.NewClock(),
	)

	members, err := cmd.constructMembers(logger, reconfigurableSink, apiConn, backendConn, gcConn, storage, lockFactory, secretManager)
	if err != nil {
		return nil, err
	}

	members = append(members, grouper.Member{
		Name: "periodic-metrics",
		Runner: metric.PeriodicallyEmit(
			logger.Session("periodic-metrics"),
			10*time.Second,
		),
	})

	onReady := func() {
		logData := lager.Data{
			"http":  cmd.nonTLSBindAddr(),
			"debug": cmd.debugBindAddr(),
		}

		if cmd.isTLSEnabled() {
			logData["https"] = cmd.tlsBindAddr()
		}

		logger.Info("listening", logData)
	}

	onExit := func() {
		for _, closer := range []Closer{lockConn, apiConn, backendConn, gcConn, storage} {
			closer.Close()
		}

		cmd.varSourcePool.Close()
	}

	return run(grouper.NewParallel(os.Interrupt, members), onReady, onExit), nil
}

func (cmd *RunCommand) constructMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	apiConn db.Conn,
	backendConn db.Conn,
	gcConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
) ([]grouper.Member, error) {
	if cmd.TelemetryOptIn {
		url := fmt.Sprintf("http://telemetry.concourse-ci.org/?version=%s", concourse.Version)
		go func() {
			_, err := http.Get(url)
			if err != nil {
				logger.Error("telemetry-version", err)
			}
		}()
	}

	policyChecker, err := policy.Initialize(logger, cmd.Server.ClusterName, concourse.Version, cmd.PolicyCheckers.Filter)
	if err != nil {
		return nil, err
	}

	apiMembers, err := cmd.constructAPIMembers(logger, reconfigurableSink, apiConn, storage, lockFactory, secretManager, policyChecker)
	if err != nil {
		return nil, err
	}

	backendComponents, err := cmd.backendComponents(logger, backendConn, lockFactory, secretManager, policyChecker)
	if err != nil {
		return nil, err
	}

	gcComponents, err := cmd.gcComponents(logger, gcConn, lockFactory)
	if err != nil {
		return nil, err
	}

	// use backendConn so that the Component objects created by the factory uses
	// the backend connection pool when reloading.
	componentFactory := db.NewComponentFactory(backendConn)
	bus := backendConn.Bus()

	members := apiMembers
	components := append(backendComponents, gcComponents...)
	for _, c := range components {
		dbComponent, err := componentFactory.CreateOrUpdate(c.Component)
		if err != nil {
			return nil, err
		}

		componentLogger := logger.Session(c.Component.Name)

		members = append(members, grouper.Member{
			Name: c.Component.Name,
			Runner: &component.Runner{
				Logger:    componentLogger,
				Interval:  cmd.ComponentRunnerInterval,
				Component: dbComponent,
				Bus:       bus,
				Schedulable: &component.Coordinator{
					Locker:    lockFactory,
					Component: dbComponent,
					Runnable:  c.Runnable,
				},
			},
		})

		if drainable, ok := c.Runnable.(component.Drainable); ok {
			members = append(members, grouper.Member{
				Name: c.Component.Name + "-drainer",
				Runner: drainRunner{
					logger:  componentLogger.Session("drain"),
					drainer: drainable,
				},
			})
		}
	}

	return members, nil
}

func (cmd *RunCommand) constructAPIMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	dbConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
	policyChecker *policy.Checker,
) ([]grouper.Member, error) {

	httpClient, err := cmd.skyHttpClient()
	if err != nil {
		return nil, err
	}

	teamFactory := db.NewTeamFactory(dbConn, lockFactory)

	_, err = teamFactory.CreateDefaultTeamIfNotExists()
	if err != nil {
		return nil, err
	}

	err = cmd.configureAuthForDefaultTeam(teamFactory)
	if err != nil {
		return nil, err
	}

	userFactory := db.NewUserFactory(dbConn)

	resourceFactory := resource.NewResourceFactory()
	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	fetchSourceFactory := worker.NewFetchSourceFactory(dbResourceCacheFactory)
	resourceFetcher := worker.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
	)

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	dbTaskCacheFactory := db.NewTaskCacheFactory(dbConn)
	dbVolumeRepository := db.NewVolumeRepository(dbConn)
	dbWorkerFactory := db.NewWorkerFactory(dbConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return nil, err
	}

	compressionLib := compression.NewGzipCompression()
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(imageResourceFetcherFactory, compressionLib),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.BaggageclaimResponseHeaderTimeout,
		cmd.GardenRequestTimeout,
		policyChecker,
	)

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool, workerProvider, compressionLib, workerAvailabilityPollingInterval, workerStatusPublishInterval)

	credsManagers := cmd.CredentialManagers
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbResourceFactory := db.NewResourceFactory(dbConn, lockFactory)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	gcContainerDestroyer := gc.NewDestroyer(logger, dbContainerRepository, dbVolumeRepository)
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, cmd.GlobalResourceCheckTimeout)
	dbClock := db.NewClock()
	dbWall := db.NewWall(dbConn, &dbClock)

	tokenVerifier := cmd.constructTokenVerifier(httpClient)

	accessFactory := accessor.NewAccessFactory(
		cmd.SystemClaimKey,
		cmd.SystemClaimValues,
	)

	middleware := token.NewMiddleware(cmd.Auth.AuthFlags.SecureCookies)

	apiHandler, err := cmd.constructAPIHandler(
		logger,
		reconfigurableSink,
		teamFactory,
		dbPipelineFactory,
		dbJobFactory,
		dbResourceFactory,
		dbWorkerFactory,
		dbVolumeRepository,
		dbContainerRepository,
		gcContainerDestroyer,
		dbBuildFactory,
		dbCheckFactory,
		dbResourceConfigFactory,
		userFactory,
		workerClient,
		secretManager,
		credsManagers,
		accessFactory,
		dbWall,
		tokenVerifier,
		dbConn.Bus(),
		policyChecker,
	)
	if err != nil {
		return nil, err
	}

	webHandler, err := cmd.constructWebHandler(logger)
	if err != nil {
		return nil, err
	}

	authHandler, err := cmd.constructAuthHandler(
		logger,
		storage,
	)
	if err != nil {
		return nil, err
	}

	loginHandler, err := cmd.constructLoginHandler(
		logger,
		httpClient,
		middleware,
	)
	if err != nil {
		return nil, err
	}

	legacyHandler, err := cmd.constructLegacyHandler(
		logger,
	)
	if err != nil {
		return nil, err
	}

	var httpHandler, httpsHandler http.Handler
	if cmd.isTLSEnabled() {
		httpHandler = cmd.constructHTTPHandler(
			logger,

			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   webHandler,
			},

			// note: intentionally not wrapping API; redirecting is more trouble than
			// it's worth.

			// we're mainly interested in having the web UI consistently https:// -
			// API requests will likely not respect the redirected https:// URI upon
			// the next request, plus the payload will have already been sent in
			// plaintext
			apiHandler,

			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   authHandler,
			},
			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   loginHandler,
			},
			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   legacyHandler,
			},
			middleware,
		)

		httpsHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			apiHandler,
			authHandler,
			loginHandler,
			legacyHandler,
			middleware,
		)
	} else {
		httpHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			apiHandler,
			authHandler,
			loginHandler,
			legacyHandler,
			middleware,
		)
	}

	members := []grouper.Member{
		{Name: "debug", Runner: http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},
		{Name: "web", Runner: http_server.New(
			cmd.nonTLSBindAddr(),
			httpHandler,
		)},
	}

	if httpsHandler != nil {
		tlsConfig, err := cmd.tlsConfig(logger, dbConn)
		if err != nil {
			return nil, err
		}
		members = append(members, grouper.Member{Name: "web-tls", Runner: http_server.NewTLSServer(
			cmd.tlsBindAddr(),
			httpsHandler,
			tlsConfig,
		)})
	}

	return members, nil
}

func (cmd *RunCommand) backendComponents(
	logger lager.Logger,
	dbConn db.Conn,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
	policyChecker *policy.Checker,
) ([]RunnableComponent, error) {

	if cmd.Syslog.Address != "" && cmd.Syslog.Transport == "" {
		return nil, fmt.Errorf("syslog Drainer is misconfigured, cannot configure a drainer without a transport")
	}

	syslogDrainConfigured := true
	if cmd.Syslog.Address == "" {
		syslogDrainConfigured = false
	}

	teamFactory := db.NewTeamFactory(dbConn, lockFactory)

	resourceFactory := resource.NewResourceFactory()
	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	fetchSourceFactory := worker.NewFetchSourceFactory(dbResourceCacheFactory)
	resourceFetcher := worker.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
	)

	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, cmd.GlobalResourceCheckTimeout)
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbCheckableCounter := db.NewCheckableCounter(dbConn)

	alg := algorithm.New(db.NewVersionsDB(dbConn, algorithmLimitRows, schedulerCache))

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbTaskCacheFactory := db.NewTaskCacheFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	dbVolumeRepository := db.NewVolumeRepository(dbConn)
	dbWorkerFactory := db.NewWorkerFactory(dbConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return nil, err
	}

	var compressionLib compression.Compression
	if cmd.StreamingArtifactsCompression == "zstd" {
		compressionLib = compression.NewZstdCompression()
	} else {
		compressionLib = compression.NewGzipCompression()
	}
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(imageResourceFetcherFactory, compressionLib),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.BaggageclaimResponseHeaderTimeout,
		cmd.GardenRequestTimeout,
		policyChecker,
	)

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool,
		workerProvider,
		compressionLib,
		workerAvailabilityPollingInterval,
		workerStatusPublishInterval)

	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	buildContainerStrategy, err := cmd.chooseBuildContainerStrategy()
	if err != nil {
		return nil, err
	}

	engine := cmd.constructEngine(
		pool,
		workerClient,
		resourceFactory,
		teamFactory,
		dbBuildFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		secretManager,
		defaultLimits,
		buildContainerStrategy,
		lockFactory,
	)

	// In case that a user configures resource-checking-interval, but forgets to
	// configure resource-with-webhook-checking-interval, keep both checking-
	// intervals consistent. Even if both intervals are configured, there is no
	// reason webhooked resources take shorter checking interval than normal
	// resources.
	if cmd.ResourceWithWebhookCheckingInterval < cmd.ResourceCheckingInterval {
		logger.Info("update-resource-with-webhook-checking-interval",
			lager.Data{
				"oldValue": cmd.ResourceWithWebhookCheckingInterval,
				"newValue": cmd.ResourceCheckingInterval,
			})
		cmd.ResourceWithWebhookCheckingInterval = cmd.ResourceCheckingInterval
	}

	components := []RunnableComponent{
		{
			Component: atc.Component{
				Name:     atc.ComponentLidarScanner,
				Interval: cmd.LidarScannerInterval,
			},
			Runnable: lidar.NewScanner(
				logger.Session(atc.ComponentLidarScanner),
				dbCheckFactory,
				secretManager,
				cmd.GlobalResourceCheckTimeout,
				cmd.ResourceCheckingInterval,
				cmd.ResourceWithWebhookCheckingInterval,
			),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentLidarChecker,
				Interval: cmd.LidarCheckerInterval,
			},
			Runnable: lidar.NewChecker(
				logger.Session(atc.ComponentLidarChecker),
				dbCheckFactory,
				engine,
				lidar.CheckRateCalculator{
					MaxChecksPerSecond:       cmd.MaxChecksPerSecond,
					ResourceCheckingInterval: cmd.ResourceCheckingInterval,
					CheckableCounter:         dbCheckableCounter,
				},
			),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentScheduler,
				Interval: 10 * time.Second,
			},
			Runnable: scheduler.NewRunner(
				logger.Session("scheduler"),
				dbJobFactory,
				&scheduler.Scheduler{
					Algorithm: alg,
					BuildStarter: scheduler.NewBuildStarter(
						builds.NewPlanner(
							atc.NewPlanFactory(time.Now().Unix()),
						),
						alg),
				},
				cmd.JobSchedulingMaxInFlight,
			),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentBuildTracker,
				Interval: cmd.BuildTrackerInterval,
			},
			Runnable: builds.NewTracker(dbBuildFactory, engine),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentBuildReaper,
				Interval: 30 * time.Second,
			},
			Runnable: gc.NewBuildLogCollector(
				dbPipelineFactory,
				500,
				gc.NewBuildLogRetentionCalculator(
					cmd.DefaultBuildLogsToRetain,
					cmd.MaxBuildLogsToRetain,
					cmd.DefaultDaysToRetainBuildLogs,
					cmd.MaxDaysToRetainBuildLogs,
				),
				syslogDrainConfigured,
			),
		},
	}

	if syslogDrainConfigured {
		components = append(components, RunnableComponent{
			Component: atc.Component{
				Name:     atc.ComponentSyslogDrainer,
				Interval: cmd.Syslog.DrainInterval,
			},
			Runnable: syslog.NewDrainer(
				cmd.Syslog.Transport,
				cmd.Syslog.Address,
				cmd.Syslog.Hostname,
				cmd.Syslog.CACerts,
				dbBuildFactory,
			),
		})
	}

	return components, err
}

func (cmd *RunCommand) gcComponents(
	logger lager.Logger,
	gcConn db.Conn,
	lockFactory lock.LockFactory,
) ([]RunnableComponent, error) {
	dbWorkerLifecycle := db.NewWorkerLifecycle(gcConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(gcConn)
	dbContainerRepository := db.NewContainerRepository(gcConn)
	dbArtifactLifecycle := db.NewArtifactLifecycle(gcConn)
	dbCheckLifecycle := db.NewCheckLifecycle(gcConn)
	resourceConfigCheckSessionLifecycle := db.NewResourceConfigCheckSessionLifecycle(gcConn)
	dbBuildFactory := db.NewBuildFactory(gcConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbResourceConfigFactory := db.NewResourceConfigFactory(gcConn, lockFactory)
	dbPipelineLifecycle := db.NewPipelineLifecycle(gcConn, lockFactory)

	dbVolumeRepository := db.NewVolumeRepository(gcConn)

	collectors := map[string]component.Runnable{
		atc.ComponentCollectorBuilds:            gc.NewBuildCollector(dbBuildFactory),
		atc.ComponentCollectorWorkers:           gc.NewWorkerCollector(dbWorkerLifecycle),
		atc.ComponentCollectorResourceConfigs:   gc.NewResourceConfigCollector(dbResourceConfigFactory),
		atc.ComponentCollectorResourceCaches:    gc.NewResourceCacheCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorResourceCacheUses: gc.NewResourceCacheUseCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorArtifacts:         gc.NewArtifactCollector(dbArtifactLifecycle),
		atc.ComponentCollectorChecks:            gc.NewCheckCollector(dbCheckLifecycle, cmd.GC.CheckRecyclePeriod),
		atc.ComponentCollectorVolumes:           gc.NewVolumeCollector(dbVolumeRepository, cmd.GC.MissingGracePeriod),
		atc.ComponentCollectorContainers:        gc.NewContainerCollector(dbContainerRepository, cmd.GC.MissingGracePeriod, cmd.GC.HijackGracePeriod),
		atc.ComponentCollectorCheckSessions:     gc.NewResourceConfigCheckSessionCollector(resourceConfigCheckSessionLifecycle),
		atc.ComponentCollectorPipelines:         gc.NewPipelineCollector(dbPipelineLifecycle),
	}

	var components []RunnableComponent
	for collectorName, collector := range collectors {
		components = append(components, RunnableComponent{
			Component: atc.Component{
				Name:     collectorName,
				Interval: cmd.GC.Interval,
			},
			Runnable: collector,
		})
	}

	return components, nil
}

func (cmd *RunCommand) validateCustomRoles() error {
	path := cmd.ConfigRBAC.Path()
	if path == "" {
		return nil
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to open RBAC config file (%s): %w", cmd.ConfigRBAC, err)
	}

	var data map[string][]string
	if err = yaml.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("failed to parse RBAC config file (%s): %w", cmd.ConfigRBAC, err)
	}

	allKnownRoles := map[string]bool{}
	for _, roleName := range accessor.DefaultRoles {
		allKnownRoles[roleName] = true
	}

	for role, actions := range data {
		if _, ok := allKnownRoles[role]; !ok {
			return fmt.Errorf("failed to customize roles: %w", fmt.Errorf("unknown role %s", role))
		}

		for _, action := range actions {
			if _, ok := accessor.DefaultRoles[action]; !ok {
				return fmt.Errorf("failed to customize roles: %w", fmt.Errorf("unknown action %s", action))
			}
		}
	}

	return nil
}

func (cmd *RunCommand) parseCustomRoles() (map[string]string, error) {
	mapping := map[string]string{}

	path := cmd.ConfigRBAC.Path()
	if path == "" {
		return mapping, nil
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data map[string][]string
	if err = yaml.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	for role, actions := range data {
		for _, action := range actions {
			mapping[action] = role
		}
	}

	return mapping, nil
}

func workerVersion() (version.Version, error) {
	return version.NewVersionFromString(concourse.WorkerVersion)
}

func (cmd *RunCommand) secretManager(logger lager.Logger) (creds.Secrets, error) {
	var secretsFactory creds.SecretsFactory = noop.NewNoopFactory()
	for name, manager := range cmd.CredentialManagers {
		if !manager.IsConfigured() {
			continue
		}

		credsLogger := logger.Session("credential-manager", lager.Data{
			"name": name,
		})

		credsLogger.Info("configured credentials manager")

		err := manager.Init(credsLogger)
		if err != nil {
			return nil, err
		}

		err = manager.Validate()
		if err != nil {
			return nil, fmt.Errorf("credential manager '%s' misconfigured: %s", name, err)
		}

		secretsFactory, err = manager.NewSecretsFactory(credsLogger)
		if err != nil {
			return nil, err
		}

		break
	}

	result := secretsFactory.NewSecrets()
	result = creds.NewRetryableSecrets(result, cmd.CredentialManagement.RetryConfig)
	if cmd.CredentialManagement.CacheConfig.Enabled {
		result = creds.NewCachedSecrets(result, cmd.CredentialManagement.CacheConfig)
	}
	return result, nil
}

func (cmd *RunCommand) newKey() *encryption.Key {
	var newKey *encryption.Key
	if cmd.EncryptionKey.AEAD != nil {
		newKey = encryption.NewKey(cmd.EncryptionKey.AEAD)
	}
	return newKey
}

func (cmd *RunCommand) oldKey() *encryption.Key {
	var oldKey *encryption.Key
	if cmd.OldEncryptionKey.AEAD != nil {
		oldKey = encryption.NewKey(cmd.OldEncryptionKey.AEAD)
	}
	return oldKey
}

func (cmd *RunCommand) constructWebHandler(logger lager.Logger) (http.Handler, error) {
	webHandler, err := web.NewHandler(logger)
	if err != nil {
		return nil, err
	}
	return metric.WrapHandler(logger, "web", webHandler), nil
}

func (cmd *RunCommand) skyHttpClient() (*http.Client, error) {
	httpClient := http.DefaultClient

	if cmd.isTLSEnabled() {
		certpool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		if !cmd.LetsEncrypt.Enable {
			cert, err := tls.LoadX509KeyPair(string(cmd.TLSCert), string(cmd.TLSKey))
			if err != nil {
				return nil, err
			}

			x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				return nil, err
			}

			certpool.AddCert(x509Cert)
		}

		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certpool,
			},
		}
	} else {
		httpClient.Transport = http.DefaultTransport
	}

	httpClient.Transport = mitmRoundTripper{
		RoundTripper: httpClient.Transport,

		SourceHost: cmd.ExternalURL.URL.Host,
		TargetURL:  cmd.DefaultURL().URL,
	}

	return httpClient, nil
}

type mitmRoundTripper struct {
	http.RoundTripper

	SourceHost string
	TargetURL  *url.URL
}

func (tripper mitmRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == tripper.SourceHost {
		req.URL.Scheme = tripper.TargetURL.Scheme
		req.URL.Host = tripper.TargetURL.Host
	}

	return tripper.RoundTripper.RoundTrip(req)
}

func (cmd *RunCommand) tlsConfig(logger lager.Logger, dbConn db.Conn) (*tls.Config, error) {
	var tlsConfig *tls.Config
	tlsConfig = atc.DefaultTLSConfig()

	if cmd.isTLSEnabled() {
		tlsLogger := logger.Session("tls-enabled")
		if cmd.LetsEncrypt.Enable {
			tlsLogger.Debug("using-autocert-manager")

			cache, err := newDbCache(dbConn)
			if err != nil {
				return nil, err
			}
			m := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				Cache:      cache,
				HostPolicy: autocert.HostWhitelist(cmd.ExternalURL.URL.Hostname()),
				Client:     &acme.Client{DirectoryURL: cmd.LetsEncrypt.ACMEURL.String()},
			}
			tlsConfig.NextProtos = append(tlsConfig.NextProtos, acme.ALPNProto)
			tlsConfig.GetCertificate = m.GetCertificate
		} else {
			tlsLogger.Debug("loading-tls-certs")
			cert, err := tls.LoadX509KeyPair(string(cmd.TLSCert), string(cmd.TLSKey))
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}
	return tlsConfig, nil
}

func (cmd *RunCommand) parseDefaultLimits() (atc.ContainerLimits, error) {
	return atc.ParseContainerLimits(map[string]interface{}{
		"cpu":    cmd.DefaultCpuLimit,
		"memory": cmd.DefaultMemoryLimit,
	})
}

func (cmd *RunCommand) defaultBindIP() net.IP {
	URL := cmd.BindIP.String()
	if URL == "0.0.0.0" {
		URL = "127.0.0.1"
	}

	return net.ParseIP(URL)
}

func (cmd *RunCommand) DefaultURL() flag.URL {
	return flag.URL{
		URL: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", cmd.defaultBindIP().String(), cmd.BindPort),
		},
	}
}

func run(runner ifrit.Runner, onReady func(), onExit func()) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		process := ifrit.Background(runner)

		subExited := process.Wait()
		subReady := process.Ready()

		for {
			select {
			case <-subReady:
				onReady()
				close(ready)
				subReady = nil
			case err := <-subExited:
				onExit()
				return err
			case sig := <-signals:
				process.Signal(sig)
			}
		}
	})
}

func (cmd *RunCommand) validate() error {
	var errs *multierror.Error

	switch {
	case cmd.TLSBindPort == 0:
		if cmd.TLSCert != "" || cmd.TLSKey != "" || cmd.LetsEncrypt.Enable {
			errs = multierror.Append(
				errs,
				errors.New("must specify --tls-bind-port to use TLS"),
			)
		}
	case cmd.LetsEncrypt.Enable:
		if cmd.TLSCert != "" || cmd.TLSKey != "" {
			errs = multierror.Append(
				errs,
				errors.New("cannot specify --enable-lets-encrypt if --tls-cert or --tls-key are set"),
			)
		}
	case cmd.TLSCert != "" && cmd.TLSKey != "":
		if cmd.ExternalURL.URL.Scheme != "https" {
			errs = multierror.Append(
				errs,
				errors.New("must specify HTTPS external-url to use TLS"),
			)
		}
	default:
		errs = multierror.Append(
			errs,
			errors.New("must specify --tls-cert and --tls-key, or --enable-lets-encrypt to use TLS"),
		)
	}

	if err := cmd.validateCustomRoles(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

func (cmd *RunCommand) nonTLSBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
}

func (cmd *RunCommand) tlsBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.TLSBindPort)
}

func (cmd *RunCommand) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.DebugBindIP, cmd.DebugBindPort)
}

func (cmd *RunCommand) configureMetrics(logger lager.Logger) error {
	host := cmd.Metrics.HostName
	if host == "" {
		host, _ = os.Hostname()
	}

	return metric.Initialize(logger.Session("metrics"), host, cmd.Metrics.Attributes, cmd.Metrics.BufferSize)
}

func (cmd *RunCommand) constructDBConn(
	driverName string,
	logger lager.Logger,
	maxConn int,
	connectionName string,
	lockFactory lock.LockFactory,
) (db.Conn, error) {
	dbConn, err := db.Open(logger.Session("db"), driverName, cmd.Postgres.ConnectionString(), cmd.newKey(), cmd.oldKey(), connectionName, lockFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %s", err)
	}

	// Instrument with Metrics
	dbConn = metric.CountQueries(dbConn)
	metric.Databases = append(metric.Databases, dbConn)

	// Instrument with Logging
	if cmd.LogDBQueries {
		dbConn = db.Log(logger.Session("log-conn"), dbConn)
	}

	// Prepare
	dbConn.SetMaxOpenConns(maxConn)
	dbConn.SetMaxIdleConns(maxConn / 2)

	return dbConn, nil
}

type Closer interface {
	Close() error
}

func (cmd *RunCommand) constructLockConn(driverName string) (*sql.DB, error) {
	dbConn, err := sql.Open(driverName, cmd.Postgres.ConnectionString())
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxOpenConns(1)
	dbConn.SetMaxIdleConns(1)
	dbConn.SetConnMaxLifetime(0)

	return dbConn, nil
}

func (cmd *RunCommand) chooseBuildContainerStrategy() (worker.ContainerPlacementStrategy, error) {
	var strategy worker.ContainerPlacementStrategy
	if cmd.ContainerPlacementStrategy != "limit-active-tasks" && cmd.MaxActiveTasksPerWorker != 0 {
		return nil, errors.New("max-active-tasks-per-worker has only effect with limit-active-tasks strategy")
	}
	if cmd.MaxActiveTasksPerWorker < 0 {
		return nil, errors.New("max-active-tasks-per-worker must be greater or equal than 0")
	}
	switch cmd.ContainerPlacementStrategy {
	case "random":
		strategy = worker.NewRandomPlacementStrategy()
	case "fewest-build-containers":
		strategy = worker.NewFewestBuildContainersPlacementStrategy()
	case "limit-active-tasks":
		strategy = worker.NewLimitActiveTasksPlacementStrategy(cmd.MaxActiveTasksPerWorker)
	default:
		strategy = worker.NewVolumeLocalityPlacementStrategy()
	}

	return strategy, nil
}

func (cmd *RunCommand) configureAuthForDefaultTeam(teamFactory db.TeamFactory) error {
	team, found, err := teamFactory.FindTeam(atc.DefaultTeamName)
	if err != nil {
		return err
	}

	if !found {
		return errors.New("default team not found")
	}

	auth, err := cmd.Auth.MainTeamFlags.Format()
	if err != nil {
		return fmt.Errorf("default team auth not configured: %v", err)
	}

	err = team.UpdateProviderAuth(auth)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *RunCommand) constructEngine(
	workerPool worker.Pool,
	workerClient worker.Client,
	resourceFactory resource.ResourceFactory,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	secretManager creds.Secrets,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	lockFactory lock.LockFactory,
) engine.Engine {

	stepFactory := builder.NewStepFactory(
		workerPool,
		workerClient,
		resourceFactory,
		teamFactory,
		buildFactory,
		resourceCacheFactory,
		resourceConfigFactory,
		defaultLimits,
		strategy,
		lockFactory,
		cmd.EnableBuildRerunWhenWorkerDisappears,
	)

	stepBuilder := builder.NewStepBuilder(
		stepFactory,
		builder.NewDelegateFactory(),
		cmd.ExternalURL.String(),
		secretManager,
		cmd.varSourcePool,
		cmd.EnableRedactSecrets,
	)

	return engine.NewEngine(stepBuilder)
}

func (cmd *RunCommand) constructHTTPHandler(
	logger lager.Logger,
	webHandler http.Handler,
	apiHandler http.Handler,
	authHandler http.Handler,
	loginHandler http.Handler,
	legacyHandler http.Handler,
	middleware token.Middleware,
) http.Handler {

	csrfHandler := auth.CSRFValidationHandler(
		apiHandler,
		middleware,
	)

	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", csrfHandler)
	webMux.Handle("/sky/issuer/", authHandler)
	webMux.Handle("/sky/", loginHandler)
	webMux.Handle("/auth/", legacyHandler)
	webMux.Handle("/login", legacyHandler)
	webMux.Handle("/logout", legacyHandler)
	webMux.Handle("/", webHandler)

	httpHandler := wrappa.LoggerHandler{
		Logger: logger,

		Handler: wrappa.SecurityHandler{
			XFrameOptions: cmd.Server.XFrameOptions,

			// proxy Authorization header to/from auth cookie,
			// to support auth from JS (EventSource) and custom JWT auth
			Handler: auth.WebAuthHandler{
				Handler:    webMux,
				Middleware: middleware,
			},
		},
	}

	return httpHandler
}

func (cmd *RunCommand) constructLegacyHandler(
	logger lager.Logger,
) (http.Handler, error) {
	return legacyserver.NewLegacyServer(&legacyserver.LegacyConfig{
		Logger: logger.Session("legacy"),
	})
}

func (cmd *RunCommand) constructAuthHandler(
	logger lager.Logger,
	storage storage.Storage,
) (http.Handler, error) {

	issuerPath, _ := url.Parse("/sky/issuer")
	redirectPath, _ := url.Parse("/sky/callback")

	issuerURL := cmd.ExternalURL.URL.ResolveReference(issuerPath)
	redirectURL := cmd.ExternalURL.URL.ResolveReference(redirectPath)

	// Add public fly client
	cmd.Auth.AuthFlags.Clients[flyClientID] = flyClientSecret

	dexServer, err := dexserver.NewDexServer(&dexserver.DexConfig{
		Logger:      logger.Session("dex"),
		Users:       cmd.Auth.AuthFlags.LocalUsers,
		Clients:     cmd.Auth.AuthFlags.Clients,
		Expiration:  cmd.Auth.AuthFlags.Expiration,
		IssuerURL:   issuerURL.String(),
		RedirectURL: redirectURL.String(),
		WebHostURL:  "/sky/issuer",
		SigningKey:  cmd.Auth.AuthFlags.SigningKey.PrivateKey,
		Storage:     storage,
	})
	if err != nil {
		return nil, err
	}

	return dexServer, nil
}

func (cmd *RunCommand) constructLoginHandler(
	logger lager.Logger,
	httpClient *http.Client,
	middleware token.Middleware,
) (http.Handler, error) {

	authPath, _ := url.Parse("/sky/issuer/auth")
	tokenPath, _ := url.Parse("/sky/issuer/token")
	redirectPath, _ := url.Parse("/sky/callback")

	authURL := cmd.ExternalURL.URL.ResolveReference(authPath)
	tokenURL := cmd.ExternalURL.URL.ResolveReference(tokenPath)
	redirectURL := cmd.ExternalURL.URL.ResolveReference(redirectPath)

	endpoint := oauth2.Endpoint{
		AuthURL:   authURL.String(),
		TokenURL:  tokenURL.String(),
		AuthStyle: oauth2.AuthStyleInHeader,
	}

	oauth2Config := &oauth2.Config{
		Endpoint:     endpoint,
		ClientID:     cmd.Server.ClientID,
		ClientSecret: cmd.Server.ClientSecret,
		RedirectURL:  redirectURL.String(),
		Scopes:       []string{"openid", "profile", "email", "federated:id", "groups"},
	}

	skyServer, err := skyserver.NewSkyServer(&skyserver.SkyConfig{
		Logger:          logger.Session("sky"),
		TokenMiddleware: middleware,
		OAuthConfig:     oauth2Config,
		HTTPClient:      httpClient,
	})
	if err != nil {
		return nil, err
	}

	return skyserver.NewSkyHandler(skyServer), nil
}

func (cmd *RunCommand) constructTokenVerifier(httpClient *http.Client) accessor.TokenVerifier {

	publicKeyPath, _ := url.Parse("/sky/issuer/keys")
	publicKeyURL := cmd.ExternalURL.URL.ResolveReference(publicKeyPath)

	validClients := []string{flyClientID}
	for clientId, _ := range cmd.Auth.AuthFlags.Clients {
		validClients = append(validClients, clientId)
	}

	return accessor.NewVerifier(httpClient, publicKeyURL, validClients)
}

func (cmd *RunCommand) constructAPIHandler(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	teamFactory db.TeamFactory,
	dbPipelineFactory db.PipelineFactory,
	dbJobFactory db.JobFactory,
	dbResourceFactory db.ResourceFactory,
	dbWorkerFactory db.WorkerFactory,
	dbVolumeRepository db.VolumeRepository,
	dbContainerRepository db.ContainerRepository,
	gcContainerDestroyer gc.Destroyer,
	dbBuildFactory db.BuildFactory,
	dbCheckFactory db.CheckFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	dbUserFactory db.UserFactory,
	workerClient worker.Client,
	secretManager creds.Secrets,
	credsManagers creds.Managers,
	accessFactory accessor.AccessFactory,
	dbWall db.Wall,
	tokenVerifier accessor.TokenVerifier,
	notifications db.NotificationsBus,
	policyChecker *policy.Checker,
) (http.Handler, error) {

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(teamFactory)
	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)
	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)
	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	rejectArchivedHandlerFactory := pipelineserver.NewRejectArchivedHandlerFactory(teamFactory)

	aud := auditor.NewAuditor(
		cmd.Auditor.EnableBuildAuditLog,
		cmd.Auditor.EnableContainerAuditLog,
		cmd.Auditor.EnableJobAuditLog,
		cmd.Auditor.EnablePipelineAuditLog,
		cmd.Auditor.EnableResourceAuditLog,
		cmd.Auditor.EnableSystemAuditLog,
		cmd.Auditor.EnableTeamAuditLog,
		cmd.Auditor.EnableWorkerAuditLog,
		cmd.Auditor.EnableVolumeAuditLog,
		logger,
	)

	batcher := accessor.NewBatcher(
		logger,
		dbUserFactory,
		BatcherInterval,
		100,
	)

	cacher := accessor.NewCacher(
		logger,
		notifications,
		teamFactory,
		time.Minute,
		time.Minute,
	)

	customRoles, err := cmd.parseCustomRoles()
	if err != nil {
		return nil, err
	}

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewConcurrentRequestLimitsWrappa(
			logger,
			wrappa.NewConcurrentRequestPolicy(cmd.ConcurrentRequestLimits),
		),
		wrappa.NewAPIMetricsWrappa(logger),
		wrappa.NewPolicyCheckWrappa(logger, policychecker.NewApiPolicyChecker(policyChecker)),
		wrappa.NewAPIAuthWrappa(
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
			checkWorkerTeamAccessHandlerFactory,
		),
		wrappa.NewRejectArchivedWrappa(rejectArchivedHandlerFactory),
		wrappa.NewConcourseVersionWrappa(concourse.Version),
		wrappa.NewAccessorWrappa(
			logger,
			accessFactory,
			tokenVerifier,
			cacher,
			batcher,
			aud,
			customRoles,
		),
		wrappa.NewCompressionWrappa(logger),
	}

	return api.NewHandler(
		logger,
		cmd.ExternalURL.String(),
		cmd.Server.ClusterName,
		apiWrapper,

		teamFactory,
		dbPipelineFactory,
		dbJobFactory,
		dbResourceFactory,
		dbWorkerFactory,
		dbVolumeRepository,
		dbContainerRepository,
		gcContainerDestroyer,
		dbBuildFactory,
		dbCheckFactory,
		resourceConfigFactory,
		dbUserFactory,

		buildserver.NewEventHandler,

		workerClient,

		reconfigurableSink,

		cmd.isTLSEnabled(),

		cmd.CLIArtifactsDir.Path(),
		concourse.Version,
		concourse.WorkerVersion,
		secretManager,
		cmd.varSourcePool,
		credsManagers,
		containerserver.NewInterceptTimeoutFactory(cmd.InterceptIdleTimeout),
		time.Minute,
		dbWall,
		clock.NewClock(),

		cmd.EnableArchivePipeline,
	)
}

type tlsRedirectHandler struct {
	matchHostname string
	externalHost  string
	baseHandler   http.Handler
}

func (h tlsRedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.Host, h.matchHostname) && (r.Method == "GET" || r.Method == "HEAD") {
		u := url.URL{
			Scheme:   "https",
			Host:     h.externalHost,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}

		http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
	} else {
		h.baseHandler.ServeHTTP(w, r)
	}
}

func (cmd *RunCommand) isTLSEnabled() bool {
	return cmd.TLSBindPort != 0
}

type drainRunner struct {
	logger  lager.Logger
	drainer component.Drainable
}

func (runner drainRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	<-signals
	runner.drainer.Drain(lagerctx.NewContext(context.Background(), runner.logger))
	return nil
}

type RunnableComponent struct {
	atc.Component
	component.Runnable
}
