package atccmd

import (
	"time"

	"github.com/concourse/flag"
	"github.com/spf13/cobra"
)

// XXX !! IMPORTANT !!
// These flags exist purely for backwards compatibility. Any new fields added
// to the RunConfig will NOT need to be added here because we do not want to
// support new fields as flags.

func InitializeATCFlagsDEPRECATED(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringVar(&flags.Logger.LogLevel, "log-level", CmdDefaults.Logger.LogLevel, "Minimum level of logs to see.")
	c.Flags().IPVar(&flags.BindIP, "bind-ip", CmdDefaults.BindIP, "IP address on which to listen for web traffic.")
	c.Flags().Uint16Var(&flags.BindPort, "bind-port", CmdDefaults.BindPort, "Port on which to listen for HTTP traffic.")
	c.Flags().Var(&flags.ExternalURL, "external-url", "URL used to reach any ATC from the outside world.")
	InitializeTLSFlags(c, flags)

	InitializeAuthFlags(c, flags)
	InitializeServerFlags(c, flags)
	InitializeSystemClaimFlags(c, flags)
	InitializeLetsEncryptFlags(c, flags)
	c.Flags().Var(&flags.ConfigRBAC, "config-rbac", "Customize RBAC role-action mapping.")
	InitializePolicyFlags(c, flags)
	c.Flags().Var(&flags.DisplayUserIdPerConnector, "display-user-id-per-connector", "Define how to display user ID for each authentication connector. Format is <connector>:<fieldname>. Valid field names are user_id, name, username and email, where name maps to claims field username, and username maps to claims field preferred username")

	InitializeDatabaseFlags(c, flags)

	InitializeSecretRetryFlags(c, flags)
	InitializeCachedSecretsFlags(c, flags)
	InitializeManagerFlags(c, flags)

	c.Flags().DurationVar(&flags.ComponentRunnerInterval, "component-runner-interval", CmdDefaults.ComponentRunnerInterval, "Interval on which runners are kicked off for builds, locks, scans, and checks")
	c.Flags().DurationVar(&flags.BuildTrackerInterval, "build-tracker-interval", CmdDefaults.BuildTrackerInterval, "Interval on which to run build tracking.")

	InitializeResourceCheckingFlags(c, flags)
	InitializeJobSchedulingFlags(c, flags)
	InitializeRuntimeFlags(c, flags)

	InitializeGCFlags(c, flags)
	InitializeBuildLogRetentionFlags(c, flags)

	InitializeDebugFlags(c, flags)
	InitializeLogFlags(c, flags)
	InitializeMetricsFlags(c, flags)
	InitializeMetricsEmitterFlags(c, flags)
	InitializeTracingFlags(c, flags)
	InitializeAuditorFlags(c, flags)
	InitializeSyslogFlags(c, flags)

	c.Flags().IntVar(&flags.DefaultCpuLimit.Limit, "default-task-cpu-limit", 0, "Default max number of cpu shares per task, 0 means unlimited")

	c.Flags().StringVar(&flags.DefaultMemoryLimit.Limit, "default-task-memory-limit", "", "Default maximum memory per task, 0 means unlimited")

	c.Flags().DurationVar(&flags.InterceptIdleTimeout, "intercept-idle-timeout", CmdDefaults.InterceptIdleTimeout, "Length of time for a intercepted session to be idle before terminating.")

	c.Flags().Var(&flags.CLIArtifactsDir, "cli-artifacts-dir", "Directory containing downloadable CLI binaries.")
	c.Flags().Var(&flags.WebPublicDir, "web-public-dir", "Web public/ directory to serve live for local development.")

	c.Flags().Var(&flags.BaseResourceTypeDefaults, "base-resource-type-defaults", "Base resource type defaults")

	c.Flags().BoolVar(&flags.TelemetryOptIn, "telemetry-opt-in", false, "Enable anonymous concourse version reporting.")
	c.Flags().MarkHidden("telemetry-opt-in")

	InitializeExperimentalFlags(c, flags)
}

func InitializeTLSFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().Uint16Var(&flags.TLS.BindPort, "tls-bind-port", 0, "Port on which to listen for HTTPS traffic.")
	c.Flags().Var(&flags.TLS.Cert, "tls-cert", "File containing an SSL certificate.")
	c.Flags().Var(&flags.TLS.Key, "tls-key", "File containing an RSA private key, used to encrypt HTTPS traffic.")
	c.Flags().Var(&flags.TLS.CaCert, "tls-ca-cert", "File containing the client CA certificate, enables mTLS")
}

func InitializeLetsEncryptFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().BoolVar(&flags.LetsEncrypt.Enable, "enable-lets-encrypt", false, "Automatically configure TLS certificates via Let's Encrypt/ACME.")
	c.Flags().Var(&flags.LetsEncrypt.ACMEURL, "lets-encrypt-acme-url", "URL of the ACME CA directory endpoint.")
}

func InitializePostgresFlags(c *cobra.Command, flags *flag.PostgresConfig) {
	c.Flags().StringVar(&flags.Host, "postgres-host", CmdDefaults.Database.Postgres.Host, "The host to connect to.")
	c.Flags().Uint16Var(&flags.Port, "postgres-port", CmdDefaults.Database.Postgres.Port, "The port to connect to.")
	c.Flags().StringVar(&flags.Socket, "postgres-socket", "", "Path to a UNIX domain socket to connect to.")
	c.Flags().StringVar(&flags.User, "postgres-user", "", "The user to sign in as.")
	c.Flags().StringVar(&flags.Password, "postgres-password", "", "The user's password.")
	c.Flags().StringVar(&flags.SSLMode, "postgres-sslmode", CmdDefaults.Database.Postgres.SSLMode, "Whether or not to use SSL.")
	c.Flags().Var(&flags.CACert, "postgres-ca-cert", "CA cert file location, to verify when connecting with SSL.")
	c.Flags().Var(&flags.ClientCert, "postgres-client-cert", "Client cert file location.")
	c.Flags().Var(&flags.ClientKey, "postgres-client-key", "Client key file location.")
	c.Flags().DurationVar(&flags.ConnectTimeout, "postgres-connect-timeout", CmdDefaults.Database.Postgres.ConnectTimeout, "Dialing timeout. (0 means wait indefinitely)")
	c.Flags().StringVar(&flags.Database, "postgres-database", CmdDefaults.Database.Postgres.Database, "The name of the database to use.")
}

func InitializeConnectorFlags(c *cobra.Command, flags *RunConfig) {
	// Bitbucket Cloud
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.BitbucketCloud.ClientID, "bitbucket-cloud-client-id", "", "(Required) Client id")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.BitbucketCloud.ClientSecret, "bitbucket-cloud-client-secret", "", "(Required) Client secret")

	// CF
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.CF.ClientID, "cf-client-id", "", "(Required) Client id")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.CF.ClientSecret, "cf-client-secret", "", "(Required) Client secret")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.CF.APIURL, "cf-api-url", "", "(Required) The base API URL of your CF deployment. It will use this information to discover information about the authentication provider.")
	c.Flags().Var(&flags.Auth.AuthFlags.Connectors.CF.CACerts, "cf-ca-cert", "CA Certificate")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.CF.InsecureSkipVerify, "cf-skip-ssl-validation", false, "Skip SSL validation")

	// Github
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Github.ClientID, "github-client-id", "", "(Required) Client id")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Github.ClientSecret, "github-client-secret", "", "(Required) Client secret")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Github.Host, "github-host", "", "Hostname of GitHub Enterprise deployment (No scheme, No trailing slash)")
	c.Flags().Var(&flags.Auth.AuthFlags.Connectors.Github.CACert, "github-ca-cert", "CA certificate of GitHub Enterprise deployment")

	// Gitlab
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Gitlab.ClientID, "gitlab-client-id", "", "(Required) Client id")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Gitlab.ClientSecret, "gitlab-client-secret", "", "(Required) Client secret")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Gitlab.Host, "gitlab-host", "", "Hostname of Gitlab Enterprise deployment (Include scheme, No trailing slash)")

	// LDAP
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.DisplayName, "ldap-display-name", "", "The auth provider name displayed to users on the login page")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.Host, "ldap-host", "", "(Required) The host and optional port of the LDAP server. If port isn't supplied, it will be guessed based on the TLS configuration. 389 or 636.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.BindDN, "ldap-bind-dn", "", "(Required) Bind DN for searching LDAP users and groups. Typically this is a read-only user.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.BindPW, "ldap-bind-pw", "", "(Required) Bind Password for the user specified by 'bind-dn'")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.LDAP.InsecureNoSSL, "ldap-insecure-no-ssl", false, "Required if LDAP host does not use TLS.")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.LDAP.InsecureSkipVerify, "ldap-insecure-skip-verify", false, "Skip certificate verification")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.LDAP.StartTLS, "ldap-start-tls", false, "Start on insecure port, then negotiate TLS")
	c.Flags().Var(&flags.Auth.AuthFlags.Connectors.LDAP.CACert, "ldap-ca-cert", "CA certificate")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UsernamePrompt, "username-prompt", "", "The prompt when logging in through the UI when --password-connector=ldap. Defaults to 'Username'.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.BaseDN, "ldap-user-search-base-dn", "", "BaseDN to start the search from. For example 'cn=users,dc=example,dc=com'")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.Filter, "ldap-user-search-filter", "", "Optional filter to apply when searching the directory. For example '(objectClass=person)'")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.Username, "ldap-user-search-username", "", "Attribute to match against the inputted username. This will be translated and combined with the other filter as '(<attr>=<username>)'.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.Scope, "ldap-user-search-scope", "", "Can either be: 'sub' - search the whole sub tree or 'one' - only search one level. Defaults to 'sub'.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.IDAttr, "ldap-user-search-id-attr", "", "A mapping of attributes on the user entry to claims. Defaults to 'uid'.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.EmailAttr, "ldap-user-search-email-attr", "", "A mapping of attributes on the user entry to claims. Defaults to 'mail'.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.NameAttr, "ldap-user-search-name-attr", "", "A mapping of attributes on the user entry to claims.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.UserSearch.BaseDN, "ldap-group-search-base-dn", "", "BaseDN to start the search from. For example 'cn=groups,dc=example,dc=com'")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.GroupSearch.Filter, "ldap-group-search-filter", "", "Optional filter to apply when searching the directory. For example '(objectClass=posixGroup)'")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.GroupSearch.Scope, "ldap-group-search-scope", "", "Can either be: 'sub' - search the whole sub tree or 'one' - only search one level. Defaults to 'sub'.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.GroupSearch.UserAttr, "ldap-group-search-user-attr", "", "Adds an additional requirement to the filter that an attribute in the group match the user's attribute value. The exact filter being added is: (<groupAttr>=<userAttr value>)")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.GroupSearch.GroupAttr, "ldap-group-search-group-attr", "", "Adds an additional requirement to the filter that an attribute in the group match the user's attribute value. The exact filter being added is: (<groupAttr>=<userAttr value>)")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.LDAP.GroupSearch.NameAttr, "ldap-group-search-name-attr", "", "The attribute of the group that represents its name.")

	// Microsoft
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Microsoft.ClientID, "microsoft-client-id", "", "(Required) Client id")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Microsoft.ClientSecret, "microsoft-client-secret", "", "(Required) Client secret")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.Microsoft.Tenant, "microsoft-tenant", "", "Microsoft Tenant limitation (common, consumers, organizations, tenant name or tenant uuid)")
	c.Flags().StringSliceVar(&flags.Auth.AuthFlags.Connectors.Microsoft.Groups, "microsoft-groups", nil, "Allowed Active Directory Groups")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.Microsoft.OnlySecurityGroups, "microsoft-only-security-groups", false, "Only fetch security groups")

	// OAuth
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.DisplayName, "oauth-display-name", "", "The auth provider name displayed to users on the login page")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.ClientID, "oauth-client-id", "", "(Required) Client id")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.ClientSecret, "oauth-client-secret", "", "(Required) Client secret")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.AuthURL, "oauth-auth-url", "", "(Required) Authorization URL")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.TokenURL, "oauth-token-url", "", "(Required) Token URL")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.UserInfoURL, "oauth-userinfo-url", "", "(Required) UserInfo URL")
	c.Flags().StringSliceVar(&flags.Auth.AuthFlags.Connectors.OAuth.Scopes, "oauth-scope", nil, "Any additional scopes that need to be requested during authorization")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.GroupsKey, "oauth-groups-key", CmdDefaults.Auth.AuthFlags.Connectors.OAuth.GroupsKey, "The groups key indicates which claim to use to map external groups to Concourse teams.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.UserIDKey, "oauth-user-id-key", CmdDefaults.Auth.AuthFlags.Connectors.OAuth.UserIDKey, "The user id key indicates which claim to use to map an external user id to a Concourse user id.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OAuth.UserNameKey, "oauth-user-name-key", CmdDefaults.Auth.AuthFlags.Connectors.OAuth.UserNameKey, "The user name key indicates which claim to use to map an external user name to a Concourse user name.")
	c.Flags().Var(&flags.Auth.AuthFlags.Connectors.OAuth.CACerts, "oauth-ca-cert", "CA Certificate")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.OAuth.InsecureSkipVerify, "oauth-skip-ssl-validation", false, "Skip SSL validation")

	// OIDC
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OIDC.DisplayName, "oidc-display-name", "", "The auth provider name displayed to users on the login page")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OIDC.Issuer, "oidc-issuer", "", "(Required) An OIDC issuer URL that will be used to discover provider configuration using the .well-known/openid-configuration")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OIDC.ClientID, "oidc-client-id", "", "(Required) Client id")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OIDC.ClientSecret, "oidc-client-secret", "", "(Required) Client secret")
	c.Flags().StringSliceVar(&flags.Auth.AuthFlags.Connectors.OIDC.Scopes, "oidc-scope", nil, "Any additional scopes of [openid] that need to be requested during authorization. Default to [openid, profile, email].")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OIDC.GroupsKey, "oidc-groups-key", CmdDefaults.Auth.AuthFlags.Connectors.OIDC.GroupsKey, "The groups key indicates which claim to use to map external groups to Concourse teams.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.OIDC.UserNameKey, "oidc-user-name-key", CmdDefaults.Auth.AuthFlags.Connectors.OIDC.UserNameKey, "The user name key indicates which claim to use to map an external user name to a Concourse user name.")
	c.Flags().StringSliceVar(&flags.Auth.AuthFlags.Connectors.OIDC.HostedDomains, "oidc-hosted-domains", nil, "List of whitelisted domains when using Google, only users from a listed domain will be allowed to log in")
	c.Flags().Var(&flags.Auth.AuthFlags.Connectors.OIDC.CACerts, "oidc-ca-cert", "CA Certificate")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.OIDC.InsecureSkipVerify, "oidc-skip-ssl-validation", false, "Skip SSL validation")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.OIDC.DisableGroups, "oidc-disable-groups", false, "Disable OIDC groups claims")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.OIDC.InsecureSkipEmailVerified, "oidc-skip-email-verified-validation", false, "Ignore the email_verified claim from the upstream provider, treating all users as if email_verified were true.")

	// SAML
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.DisplayName, "saml-display-name", "", "The auth provider name displayed to users on the login page")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.SsoURL, "saml-sso-url", "", "(Required) SSO URL used for POST value")
	c.Flags().Var(&flags.Auth.AuthFlags.Connectors.SAML.CACert, "saml-ca-cert", "(Required) CA Certificate")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.EntityIssuer, "saml-entity-issuer", "", "Manually specify dex's Issuer value.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.SsoIssuer, "saml-sso-issuer", "", "Issuer value expected in the SAML response.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.UsernameAttr, "saml-username-attr", CmdDefaults.Auth.AuthFlags.Connectors.SAML.UsernameAttr, "The user name indicates which claim to use to map an external user name to a Concourse user name.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.EmailAttr, "saml-email-attr", CmdDefaults.Auth.AuthFlags.Connectors.SAML.EmailAttr, "The email indicates which claim to use to map an external user email to a Concourse user email.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.GroupsAttr, "saml-groups-attr", CmdDefaults.Auth.AuthFlags.Connectors.SAML.GroupsAttr, "The groups key indicates which attribute to use to map external groups to Concourse teams.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.GroupsDelim, "saml-groups-delim", "", "If specified, groups are returned as string, this delimiter will be used to split the group string.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.Connectors.SAML.NameIDPolicyFormat, "saml-name-id-policy-format", "", "Requested format of the NameID. The NameID value is is mapped to the ID Token 'sub' claim.")
	c.Flags().BoolVar(&flags.Auth.AuthFlags.Connectors.SAML.InsecureSkipVerify, "saml-skip-ssl-validation", false, "Skip SSL validation")
}

func InitializeTeamConnectorsFlags(c *cobra.Command, flags *RunConfig) {
	// Bitbucket
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.BitbucketCloud.Users, "main-team-bitbucket-cloud-user", nil, "A whitelisted Bitbucket Cloud user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.BitbucketCloud.Teams, "main-team-bitbucket-cloud-team", nil, "A whitelisted Bitbucket Cloud team, ex.TEAM_NAME")

	// CF
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.Users, "main-team-cf-user", nil, "A whitelisted CloudFoundry user ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.Orgs, "main-team-cf-org", nil, "A whitelisted CloudFoundry org, ex.ORG_NAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.Spaces, "main-team-cf-space", nil, "(Deprecated) A whitelisted CloudFoundry space for users with the 'developer' role, ex.ORG_NAME:SPACE_NAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.SpacesAll, "main-team-cf-space-with-any-role", nil, "A whitelisted CloudFoundry space for users with any role, ex.ORG_NAME:SPACE_NAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.SpacesDeveloper, "main-team-cf-space-with-developer-role", nil, "A whitelisted CloudFoundry space for users with the 'developer' role, ex.ORG_NAME:SPACE_NAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.SpacesAuditor, "main-team-cf-space-with-auditor-role", nil, "A whitelisted CloudFoundry space for users with the 'auditor' role, ex.ORG_NAME:SPACE_NAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.SpacesManager, "main-team-cf-space-with-manager-role", nil, "A whitelisted CloudFoundry space for users with the 'manager' role, ex.ORG_NAME:SPACE_NAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.CF.SpaceGuids, "main-team-cf-space-guid", nil, "A whitelisted CloudFoundry space guid, ex.SPACE_GUID")

	// Github
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.Github.Users, "main-team-github-user", nil, "A whitelisted GitHub user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.Github.Orgs, "main-team-github-org", nil, "A whitelisted GitHub org, ex.ORG_NAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.Github.Teams, "main-team-github-team", nil, "A whitelisted GitHub team,  ex.ORG_NAME:TEAM_NAME")

	// Gitlab
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.Gitlab.Users, "main-team-gitlab-user", nil, "A whitelisted GitLab user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.Gitlab.Groups, "main-team-gitlab-group", nil, "A whitelisted GitLab group, ex.GROUP_NAME")

	// LDAP
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.LDAP.Users, "main-team-ldap-user", nil, "A whitelisted LDAP user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.LDAP.Groups, "main-team-ldap-group", nil, "A whitelisted LDAP group, ex.GROUP_NAME")

	// Microsoft
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.Microsoft.Users, "main-team-microsoft-user", nil, "A whitelisted Microsoft user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.Microsoft.Groups, "main-team-microsoft-group", nil, "A whitelisted Microsoft group, ex.GROUP_NAME")

	// OAuth
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.OAuth.Users, "main-team-oauth-user", nil, "A whitelisted OAuth2 user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.OAuth.Groups, "main-team-oauth-group", nil, "A whitelisted OAuth2 group, ex.GROUP_NAME")

	// OIDC
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.OIDC.Users, "main-team-oidc-user", nil, "A whitelisted OIDC user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.OIDC.Groups, "main-team-oidc-group", nil, "A whitelisted OIDC group, ex.GROUP_NAME")

	// SAML
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.SAML.Users, "main-team-saml-user", nil, "A whitelisted SAML user, ex.USERNAME")
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.TeamConnectors.SAML.Groups, "main-team-saml-group", nil, "A whitelisted SAML group, ex.GROUP_NAME")
}

func InitializeDatabaseFlags(c *cobra.Command, flags *RunConfig) {
	InitializePostgresFlags(c, &flags.Database.Postgres)

	c.Flags().Var(&flags.Database.ConcurrentRequestLimits, "concurrent-request-limit", "Limit the number of concurrent requests to an API endpoint (Example: ListAllJobs:5)")
	c.Flags().IntVar(&flags.Database.APIMaxOpenConnections, "api-max-conns", CmdDefaults.Database.APIMaxOpenConnections, "The maximum number of open connections for the api connection pool.")
	c.Flags().IntVar(&flags.Database.BackendMaxOpenConnections, "backend-max-conns", CmdDefaults.Database.BackendMaxOpenConnections, "The maximum number of open connections for the backend connection pool.")
	c.Flags().Var(&flags.Database.EncryptionKey, "encryption-key", "A 16 or 32 length key used to encrypt sensitive information before storing it in the database.")
	c.Flags().Var(&flags.Database.OldEncryptionKey, "old-encryption-key", "Encryption key previously used for encrypting sensitive information. If provided without a new key, data is encrypted. If provided with a new key, data is re-encrypted.")
}

func InitializeDebugFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().IPVar(&flags.Debug.BindIP, "debug-bind-ip", CmdDefaults.Debug.BindIP, "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Debug.BindPort, "debug-bind-port", CmdDefaults.Debug.BindPort, "Port on which to listen for the pprof debugger endpoints.")
}

func InitializeResourceCheckingFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().DurationVar(&flags.ResourceChecking.ScannerInterval, "lidar-scanner-interval", CmdDefaults.ResourceChecking.ScannerInterval, "Interval on which the resource scanner will run to see if new checks need to be scheduled")

	c.Flags().DurationVar(&flags.ResourceChecking.Timeout, "global-resource-check-timeout", CmdDefaults.ResourceChecking.Timeout, "Time limit on checking for new versions of resources.")
	c.Flags().DurationVar(&flags.ResourceChecking.DefaultInterval, "resource-checking-interval", CmdDefaults.ResourceChecking.DefaultInterval, "Interval on which to check for new versions of resources.")
	c.Flags().DurationVar(&flags.ResourceChecking.DefaultIntervalWithWebhook, "resource-with-webhook-checking-interval", CmdDefaults.ResourceChecking.DefaultIntervalWithWebhook, "Interval on which to check for new versions of resources that has webhook defined.")
	c.Flags().IntVar(&flags.ResourceChecking.MaxChecksPerSecond, "max-checks-per-second", 0, "Maximum number of checks that can be started per second. If not specified, this will be calculated as (# of resources)/(resource checking interval). -1 value will remove this maximum limit of checks per second.")
}

func InitializeJobSchedulingFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().Uint64Var(&flags.JobScheduling.MaxInFlight, "job-scheduling-max-in-flight", CmdDefaults.JobScheduling.MaxInFlight, "Maximum number of jobs to be scheduling at the same time")
}

func InitializeRuntimeFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringSliceVar(&flags.Runtime.ContainerPlacementStrategyOptions.ContainerPlacementStrategy, "container-placement-strategy", CmdDefaults.Runtime.ContainerPlacementStrategyOptions.ContainerPlacementStrategy, "Method by which a worker is selected during container placement. If multiple methods are specified, they will be applied in order. Random strategy should only be used alone.")
	c.Flags().IntVar(&flags.Runtime.ContainerPlacementStrategyOptions.MaxActiveTasksPerWorker, "max-active-tasks-per-worker", CmdDefaults.Runtime.ContainerPlacementStrategyOptions.MaxActiveTasksPerWorker, "Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit.")
	c.Flags().IntVar(&flags.Runtime.ContainerPlacementStrategyOptions.MaxActiveContainersPerWorker, "max-active-containers-per-worker", CmdDefaults.Runtime.ContainerPlacementStrategyOptions.MaxActiveContainersPerWorker, "Maximum allowed number of active containers per worker. Has effect only when used with limit-active-containers placement strategy. 0 means no limit.")
	c.Flags().IntVar(&flags.Runtime.ContainerPlacementStrategyOptions.MaxActiveVolumesPerWorker, "max-active-volumes-per-worker", CmdDefaults.Runtime.ContainerPlacementStrategyOptions.MaxActiveVolumesPerWorker, "Maximum allowed number of active volumes per worker. Has effect only when used with limit-active-volumes placement strategy. 0 means no limit.")

	c.Flags().DurationVar(&flags.Runtime.BaggageclaimResponseHeaderTimeout, "baggageclaim-response-header-timeout", CmdDefaults.Runtime.BaggageclaimResponseHeaderTimeout, "How long to wait for Baggageclaim to send the response header.")
	c.Flags().StringVar(&flags.Runtime.StreamingArtifactsCompression, "streaming-artifacts-compression", CmdDefaults.Runtime.StreamingArtifactsCompression, "Compression algorithm for internal streaming.")
	c.Flags().DurationVar(&flags.Runtime.GardenRequestTimeout, "garden-request-timeout", CmdDefaults.Runtime.GardenRequestTimeout, "How long to wait for requests to Garden to complete. 0 means no timeout.")
	c.Flags().DurationVar(&flags.Runtime.P2pVolumeStreamingTimeout, "p2p-volume-streaming-timeout", CmdDefaults.Runtime.P2pVolumeStreamingTimeout, "Timeout value of p2p volume streaming")
}

func InitializeMetricsFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringVar(&flags.Metrics.HostName, "metrics-host-name", "", "Host string to attach to emitted metrics.")
	c.Flags().Var(&flags.Metrics.Attributes, "metrics-attribute", "A key-value attribute to attach to emitted metrics. Can be specified multiple times. Ex: NAME:VALUE")
	c.Flags().Uint32Var(&flags.Metrics.BufferSize, "metrics-buffer-size", CmdDefaults.Metrics.BufferSize, "The size of the buffer used in emitting event metrics.")
	c.Flags().BoolVar(&flags.Metrics.CaptureErrorMetrics, "capture-error-metrics", false, "Enable capturing of error log metrics")
}

func InitializeMetricsEmitterFlags(c *cobra.Command, flags *RunConfig) {
	// Datadog
	c.Flags().StringVar(&flags.Metrics.Emitters.Datadog.Host, "datadog-agent-host", "", "Datadog agent host to expose dogstatsd metrics")
	c.Flags().StringVar(&flags.Metrics.Emitters.Datadog.Port, "datadog-agent-port", "", "Datadog agent port to expose dogstatsd metrics")
	c.Flags().StringVar(&flags.Metrics.Emitters.Datadog.Prefix, "datadog-prefix", "", "Prefix for all metrics to easily find them in Datadog")

	// InfluxDB
	c.Flags().StringVar(&flags.Metrics.Emitters.InfluxDB.URL, "influxdb-url", "", "InfluxDB server address to emit points to.")
	c.Flags().StringVar(&flags.Metrics.Emitters.InfluxDB.Database, "influxdb-database", "", "InfluxDB database to write points to.")
	c.Flags().StringVar(&flags.Metrics.Emitters.InfluxDB.Username, "influxdb-username", "", "InfluxDB server username.")
	c.Flags().StringVar(&flags.Metrics.Emitters.InfluxDB.Password, "influxdb-password", "", "InfluxDB server password.")
	c.Flags().BoolVar(&flags.Metrics.Emitters.InfluxDB.InsecureSkipVerify, "influxdb-insecure-skip-verify", false, "Skip SSL verification when emitting to InfluxDB.")
	c.Flags().Uint32Var(&flags.Metrics.Emitters.InfluxDB.BatchSize, "influxdb-batch-size", 5000, "Number of points to batch together when emitting to InfluxDB.")
	c.Flags().DurationVar(&flags.Metrics.Emitters.InfluxDB.BatchDuration, "influxdb-batch-duration", 300*time.Second, "The duration to wait before emitting a batch of points to InfluxDB, disregarding influxdb-batch-size.")

	// Lager
	c.Flags().BoolVar(&flags.Metrics.Emitters.Lager.Enabled, "emit-to-logs", false, "Emit metrics to logs.")

	// NewRelic
	c.Flags().StringVar(&flags.Metrics.Emitters.NewRelic.AccountID, "newrelic-account-id", "", "New Relic Account ID")
	c.Flags().StringVar(&flags.Metrics.Emitters.NewRelic.APIKey, "newrelic-api-key", "", "New Relic Insights API Key")
	c.Flags().StringVar(&flags.Metrics.Emitters.NewRelic.Url, "newrelic-insights-api-url", "https://insights-collector.newrelic.com", "Base Url for insights Insert API")
	c.Flags().StringVar(&flags.Metrics.Emitters.NewRelic.ServicePrefix, "newrelic-service-prefix", "", "An optional prefix for emitted New Relic events")
	c.Flags().Uint64Var(&flags.Metrics.Emitters.NewRelic.BatchSize, "newrelic-batch-size", 2000, "Number of events to batch together before emitting")
	c.Flags().DurationVar(&flags.Metrics.Emitters.NewRelic.BatchDuration, "newrelic-batch-duration", 60*time.Second, "Length of time to wait between emitting until all currently batched events are emitted")
	c.Flags().BoolVar(&flags.Metrics.Emitters.NewRelic.DisableCompression, "newrelic-batch-disable-compression", false, "Disables compression of the batch before sending it")

	// Prometheus
	c.Flags().StringVar(&flags.Metrics.Emitters.Prometheus.BindIP, "prometheus-bind-ip", "", "IP to listen on to expose Prometheus metrics.")
	c.Flags().StringVar(&flags.Metrics.Emitters.Prometheus.BindPort, "prometheus-bind-port", "", "Port to listen on to expose Prometheus metrics.")
}

func InitializeSecretRetryFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().IntVar(&flags.CredentialManagement.RetryConfig.Attempts, "secret-retry-attempts", CmdDefaults.CredentialManagement.RetryConfig.Attempts, "The number of attempts secret will be retried to be fetched, in case a retryable error happens.")
	c.Flags().DurationVar(&flags.CredentialManagement.RetryConfig.Interval, "secret-retry-interval", CmdDefaults.CredentialManagement.RetryConfig.Interval, "The interval between secret retry retrieval attempts.")
}

func InitializeCachedSecretsFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().BoolVar(&flags.CredentialManagement.CacheConfig.Enabled, "secret-cache-enabled", false, "Enable in-memory cache for secrets")
	c.Flags().DurationVar(&flags.CredentialManagement.CacheConfig.Duration, "secret-cache-duration", CmdDefaults.CredentialManagement.CacheConfig.Duration, "If the cache is enabled, secret values will be cached for not longer than this duration (it can be less, if underlying secret lease time is smaller)")
	c.Flags().DurationVar(&flags.CredentialManagement.CacheConfig.DurationNotFound, "secret-cache-duration-notfound", CmdDefaults.CredentialManagement.CacheConfig.DurationNotFound, "If the cache is enabled, secret not found responses will be cached for this duration")
	c.Flags().DurationVar(&flags.CredentialManagement.CacheConfig.PurgeInterval, "secret-cache-purge-interval", CmdDefaults.CredentialManagement.CacheConfig.PurgeInterval, "If the cache is enabled, expired items will be removed on this interval")
}

func InitializeManagerFlags(c *cobra.Command, flags *RunConfig) {
	// Conjur
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.ConjurApplianceUrl, "conjur-appliance-url", "", "URL of the conjur instance")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.ConjurAccount, "conjur-account", "", "Conjur Account")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.ConjurCertFile, "conjur-cert-file", "", "Cert file used if conjur instance is using a self signed cert. E.g. /path/to/conjur.pem")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.ConjurAuthnLogin, "conjur-authn-login", "", "Host username. E.g host/concourse")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.ConjurAuthnApiKey, "conjur-authn-api-key", "", "Api key related to the host")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.ConjurAuthnTokenFile, "conjur-authn-token-file", "", "Token file used if conjur instance is running in k8s or iam. E.g. /path/to/token_file")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.PipelineSecretTemplate, "conjur-pipeline-secret-template", CmdDefaults.CredentialManagers.Conjur.PipelineSecretTemplate, "Conjur secret identifier template used for pipeline specific parameter")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.TeamSecretTemplate, "conjur-team-secret-template", CmdDefaults.CredentialManagers.Conjur.TeamSecretTemplate, "Conjur secret identifier template used for team specific parameter")
	c.Flags().StringVar(&flags.CredentialManagers.Conjur.SecretTemplate, "conjur-secret-template", CmdDefaults.CredentialManagers.Conjur.SecretTemplate, "Conjur secret identifier template used for full path conjur secrets")

	// CredHub
	c.Flags().StringVar(&flags.CredentialManagers.CredHub.URL, "credhub-url", "", "CredHub server address used to access secrets.")
	c.Flags().StringVar(&flags.CredentialManagers.CredHub.PathPrefix, "credhub-path-prefix", CmdDefaults.CredentialManagers.CredHub.PathPrefix, "Path under which to namespace credential lookup.")
	c.Flags().StringSliceVar(&flags.CredentialManagers.CredHub.TLS.CACerts, "credhub-ca-cert", nil, "Paths to PEM-encoded CA cert files to use to verify the CredHub server SSL cert.")
	c.Flags().StringVar(&flags.CredentialManagers.CredHub.TLS.ClientCert, "credhub-client-cert", "", "Path to the client certificate for mutual TLS authorization.")
	c.Flags().StringVar(&flags.CredentialManagers.CredHub.TLS.ClientKey, "credhub-client-key", "", "Path to the client private key for mutual TLS authorization.")
	c.Flags().BoolVar(&flags.CredentialManagers.CredHub.TLS.Insecure, "credhub-insecure-skip-verify", false, "Enable insecure SSL verification.")
	c.Flags().StringVar(&flags.CredentialManagers.CredHub.UAA.ClientId, "credhub-client-id", "", "Client ID for CredHub authorization.")
	c.Flags().StringVar(&flags.CredentialManagers.CredHub.UAA.ClientSecret, "credhub-client-secret", "", "Client secret for CredHub authorization.")

	// Dummy
	c.Flags().Var(&flags.CredentialManagers.Dummy.Vars, "dummy-creds-var", "A YAML value to expose via credential management. Can be prefixed with a team and/or pipeline to limit scope. Ex. [TEAM/[PIPELINE/]]VAR=VALUE")

	// Kubernetes
	c.Flags().BoolVar(&flags.CredentialManagers.Kubernetes.InClusterConfig, "kubernetes-in-cluster", false, "Enables the in-cluster client.")
	c.Flags().StringVar(&flags.CredentialManagers.Kubernetes.ConfigPath, "kubernetes-config-path", "", "Path to Kubernetes config when running ATC outside Kubernetes.")
	c.Flags().StringVar(&flags.CredentialManagers.Kubernetes.NamespacePrefix, "kubernetes-namespace-prefix", CmdDefaults.CredentialManagers.Kubernetes.NamespacePrefix, "Prefix to use for Kubernetes namespaces under which secrets will be looked up.")

	// AWS Secrets Manager
	c.Flags().StringVar(&flags.CredentialManagers.SecretsManager.AwsAccessKeyID, "aws-secretsmanager-access-key", "", "AWS Access key ID")
	c.Flags().StringVar(&flags.CredentialManagers.SecretsManager.AwsSecretAccessKey, "aws-secretsmanager-secret-key", "", "AWS Secret Access Key")
	c.Flags().StringVar(&flags.CredentialManagers.SecretsManager.AwsSessionToken, "aws-secretsmanager-session-token", "", "AWS Session Token")
	c.Flags().StringVar(&flags.CredentialManagers.SecretsManager.AwsRegion, "aws-secretsmanager-region", "", "AWS region to send requests to")
	c.Flags().StringVar(&flags.CredentialManagers.SecretsManager.PipelineSecretTemplate, "aws-secretsmanager-pipeline-secret-template", CmdDefaults.CredentialManagers.SecretsManager.PipelineSecretTemplate, "AWS Secrets Manager secret identifier template used for pipeline specific parameter")
	c.Flags().StringVar(&flags.CredentialManagers.SecretsManager.TeamSecretTemplate, "aws-secretsmanager-team-secret-template", CmdDefaults.CredentialManagers.SecretsManager.TeamSecretTemplate, "AWS Secrets Manager secret identifier  template used for team specific parameter")

	// AWS SSM
	c.Flags().StringVar(&flags.CredentialManagers.SSM.AwsAccessKeyID, "aws-ssm-access-key", "", "AWS Access key ID")
	c.Flags().StringVar(&flags.CredentialManagers.SSM.AwsSecretAccessKey, "aws-ssm-secret-key", "", "AWS Secret Access Key")
	c.Flags().StringVar(&flags.CredentialManagers.SSM.AwsSessionToken, "aws-ssm-session-token", "", "AWS Session Token")
	c.Flags().StringVar(&flags.CredentialManagers.SSM.AwsRegion, "aws-ssm-region", "", "AWS region to send requests to")
	c.Flags().StringVar(&flags.CredentialManagers.SSM.PipelineSecretTemplate, "aws-ssm-pipeline-secret-template", CmdDefaults.CredentialManagers.SSM.PipelineSecretTemplate, "AWS SSM parameter name template used for pipeline specific parameter")
	c.Flags().StringVar(&flags.CredentialManagers.SSM.TeamSecretTemplate, "aws-ssm-team-secret-template", CmdDefaults.CredentialManagers.SSM.TeamSecretTemplate, "AWS SSM parameter name template used for team specific parameter")

	// Vault
	c.Flags().StringVar(&flags.CredentialManagers.Vault.URL, "vault-url", "", "Vault server address used to access secrets.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.PathPrefix, "vault-path-prefix", CmdDefaults.CredentialManagers.Vault.PathPrefix, "Path under which to namespace credential lookup.")
	c.Flags().StringSliceVar(&flags.CredentialManagers.Vault.LookupTemplates, "vault-lookup-templates", CmdDefaults.CredentialManagers.Vault.LookupTemplates, "Path templates for credential lookup")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.SharedPath, "vault-shared-path", "", "Path under which to lookup shared credentials.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.Namespace, "vault-namespace", "", "Vault namespace to use for authentication and secret lookup.")
	c.Flags().DurationVar(&flags.CredentialManagers.Vault.LoginTimeout, "login-timeout", CmdDefaults.CredentialManagers.Vault.LoginTimeout, "Timeout value for Vault login.")
	c.Flags().DurationVar(&flags.CredentialManagers.Vault.QueryTimeout, "query-timeout", CmdDefaults.CredentialManagers.Vault.QueryTimeout, "Timeout value for Vault query.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.TLS.CACertFile, "vault-ca-cert", "", "Path to a PEM-encoded CA cert file to use to verify the vault server SSL cert.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.TLS.CAPath, "vault-ca-path", "", "Path to a directory of PEM-encoded CA cert files to verify the vault server SSL cert.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.TLS.ClientCertFile, "vault-client-cert", "", "Path to the client certificate for Vault authorization.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.TLS.ClientKeyFile, "vault-client-key", "", "Path to the client private key for Vault authorization.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.TLS.ServerName, "vault-server-name", "", "If set, is used to set the SNI host when connecting via TLS.")
	c.Flags().BoolVar(&flags.CredentialManagers.Vault.TLS.Insecure, "vault-insecure-skip-verify", false, "Enable insecure SSL verification.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.Auth.ClientToken, "vault-client-token", "", "Client token for accessing secrets within the Vault server.")
	c.Flags().StringVar(&flags.CredentialManagers.Vault.Auth.Backend, "vault-auth-backend", "", "Auth backend to use for logging in to Vault.")
	c.Flags().DurationVar(&flags.CredentialManagers.Vault.Auth.BackendMaxTTL, "vault-auth-backend-max-ttl", 0, "Time after which to force a re-login. If not set, the token will just be continuously renewed.")
	c.Flags().DurationVar(&flags.CredentialManagers.Vault.Auth.RetryMax, "vault-retry-max", CmdDefaults.CredentialManagers.Vault.Auth.RetryMax, "The maximum time between retries when logging in or re-authing a secret.")
	c.Flags().DurationVar(&flags.CredentialManagers.Vault.Auth.RetryInitial, "vault-retry-initial", CmdDefaults.CredentialManagers.Vault.Auth.RetryInitial, "The initial time between retries when logging in or re-authing a secret.")
	c.Flags().Var(&flags.CredentialManagers.Vault.Auth.Params, "vault-auth-param", "Paramter to pass when logging in via the backend. Can be specified multiple times. Ex.NAME:VALUE")
}

func InitializeTracingFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringVar(&flags.Tracing.ServiceName, "tracing-service-name", "concourse-web", "service name to attach to traces as metadata")
	c.Flags().Var(&flags.Tracing.Attributes, "tracing-attribute", "attributes to attach to traces as metadata")

	// Honeycomb
	c.Flags().StringVar(&flags.Tracing.Providers.Honeycomb.APIKey, "tracing-honeycomb-api-key", "", "honeycomb.io api key")
	c.Flags().StringVar(&flags.Tracing.Providers.Honeycomb.Dataset, "tracing-honeycomb-dataset", "", "honeycomb.io dataset name")

	// Jaeger
	c.Flags().StringVar(&flags.Tracing.Providers.Jaeger.Endpoint, "tracing-jaeger-endpoint", "", "jaeger http-based thrift collector")
	c.Flags().Var(&flags.Tracing.Providers.Jaeger.Tags, "tracing-jaeger-tags", "tags to add to the components")
	c.Flags().StringVar(&flags.Tracing.Providers.Jaeger.Service, "tracing-jaeger-service", CmdDefaults.Tracing.Providers.Jaeger.Service, "jaeger process service name")

	// Stackdriver
	c.Flags().StringVar(&flags.Tracing.Providers.Stackdriver.ProjectID, "tracing-stackdriver-projectid", "", "GCP's Project ID")

	// OTLP
	c.Flags().StringVar(&flags.Tracing.Providers.OTLP.Address, "tracing-otlp-address", "", "otlp address to send traces to")
	c.Flags().Var(&flags.Tracing.Providers.OTLP.Headers, "tracing-otlp-header", "headers to attach to each tracing message")
	c.Flags().BoolVar(&flags.Tracing.Providers.OTLP.UseTLS, "tracing-otlp-use-tls", false, "whether to use tls or not")
}

func InitializePolicyFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringSliceVar(&flags.PolicyCheckers.Filter.HttpMethods, "policy-check-filter-http-method", nil, "API http method to go through policy check")
	c.Flags().StringSliceVar(&flags.PolicyCheckers.Filter.Actions, "policy-check-filter-action", nil, "Actions in the list will go through policy check")
	c.Flags().StringSliceVar(&flags.PolicyCheckers.Filter.ActionsToSkip, "policy-check-filter-action-skip", nil, "Actions the list will not go through policy check")
}

func InitializeServerFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringVar(&flags.Server.XFrameOptions, "x-frame-options", CmdDefaults.Server.XFrameOptions, "The value to set for X-Frame-Options.")
	c.Flags().StringVar(&flags.Server.ContentSecurityPolicy, "content-security-policy", "frame-ancestors 'none'", "The value to set for the Content-Security-Policy header.")
	c.Flags().StringVar(&flags.Server.ClusterName, "cluster-name", "", "A name for this Concourse cluster, to be displayed on the dashboard page.")
	c.Flags().StringVar(&flags.Server.ClientID, "client-id", CmdDefaults.Server.ClientID, "Client ID to use for login flow")
	c.Flags().StringVar(&flags.Server.ClientSecret, "client-secret", "", "Client secret to use for login flow")
}

func InitializeLogFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().BoolVar(&flags.Log.DBQueries, "log-db-queries", false, "Log database queries.")
	c.Flags().BoolVar(&flags.Log.ClusterName, "log-cluster-name", false, "Log cluster name.")
}

func InitializeGCFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().DurationVar(&flags.GC.Interval, "gc-interval", CmdDefaults.GC.Interval, "Interval on which to perform garbage collection.")
	c.Flags().DurationVar(&flags.GC.OneOffBuildGracePeriod, "gc-one-off-grace-period", CmdDefaults.GC.OneOffBuildGracePeriod, "Period after which one-off build containers will be garbage-collected.")
	c.Flags().DurationVar(&flags.GC.MissingGracePeriod, "gc-missing-grace-period", CmdDefaults.GC.MissingGracePeriod, "Period after which to reap containers and volumes that were created but went missing from the worker.")
	c.Flags().DurationVar(&flags.GC.HijackGracePeriod, "gc-hijack-grace-period", CmdDefaults.GC.HijackGracePeriod, "Period after which hijacked containers will be garbage collected")
	c.Flags().DurationVar(&flags.GC.FailedGracePeriod, "gc-failed-grace-period", CmdDefaults.GC.FailedGracePeriod, "Period after which failed containers will be garbage collected")
	c.Flags().DurationVar(&flags.GC.CheckRecyclePeriod, "gc-check-recycle-period", CmdDefaults.GC.CheckRecyclePeriod, "Period after which to reap checks that are completed.")
	c.Flags().DurationVar(&flags.GC.VarSourceRecyclePeriod, "var-source-recycle-period", CmdDefaults.GC.VarSourceRecyclePeriod, "Period after which to reap var_sources that are not used.")
}

func InitializeBuildLogRetentionFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().Uint64Var(&flags.BuildLogRetention.Default, "default-build-logs-to-retain", 0, "Default build logs to retain, 0 means all")
	c.Flags().Uint64Var(&flags.BuildLogRetention.Max, "max-build-logs-to-retain", 0, "Maximum build logs to retain, 0 means not specified. Will override values configured in jobs")

	c.Flags().Uint64Var(&flags.BuildLogRetention.DefaultDays, "default-days-to-retain-build-logs", 0, "Default days to retain build logs. 0 means unlimited")
	c.Flags().Uint64Var(&flags.BuildLogRetention.MaxDays, "max-days-to-retain-build-logs", 0, "Maximum days to retain build logs, 0 means not specified. Will override values configured in jobs")
}

func InitializeAuditorFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().BoolVar(&flags.Auditor.EnableBuildAuditLog, "enable-build-auditing", false, "Enable auditing for all api requests connected to builds.")
	c.Flags().BoolVar(&flags.Auditor.EnableContainerAuditLog, "enable-container-auditing", false, "Enable auditing for all api requests connected to containers.")
	c.Flags().BoolVar(&flags.Auditor.EnableJobAuditLog, "enable-job-auditing", false, "Enable auditing for all api requests connected to jobs.")
	c.Flags().BoolVar(&flags.Auditor.EnablePipelineAuditLog, "enable-pipeline-auditing", false, "Enable auditing for all api requests connected to pipelines.")
	c.Flags().BoolVar(&flags.Auditor.EnableResourceAuditLog, "enable-resource-auditing", false, "Enable auditing for all api requests connected to resources.")
	c.Flags().BoolVar(&flags.Auditor.EnableSystemAuditLog, "enable-system-auditing", false, "Enable auditing for all api requests connected to system transactions.")
	c.Flags().BoolVar(&flags.Auditor.EnableTeamAuditLog, "enable-team-auditing", false, "Enable auditing for all api requests connected to teams.")
	c.Flags().BoolVar(&flags.Auditor.EnableWorkerAuditLog, "enable-worker-auditing", false, "Enable auditing for all api requests connected to workers.")
	c.Flags().BoolVar(&flags.Auditor.EnableVolumeAuditLog, "enable-volume-auditing", false, "Enable auditing for all api requests connected to volumes.")
}

func InitializeSyslogFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringVar(&flags.Syslog.Hostname, "syslog-hostname", CmdDefaults.Syslog.Hostname, "Client hostname with which the build logs will be sent to the syslog server.")
	c.Flags().StringVar(&flags.Syslog.Address, "syslog-address", "", "Remote syslog server address with port (Example: 0.0.0.0:514).")
	c.Flags().StringVar(&flags.Syslog.Transport, "syslog-transport", "", "Transport protocol for syslog messages (Currently supporting tcp, udp & tls).")
	c.Flags().DurationVar(&flags.Syslog.DrainInterval, "syslog-drain-interval", CmdDefaults.Syslog.DrainInterval, "Interval over which checking is done for new build logs to send to syslog server (duration measurement units are s/m/h; eg. 30s/30m/1h)")
	c.Flags().StringSliceVar(&flags.Syslog.CACerts, "syslog-ca-cert", nil, "Paths to PEM-encoded CA cert files to use to verify the Syslog server SSL cert.")
}

func InitializeAuthFlags(c *cobra.Command, flags *RunConfig) {
	// Auth Flags
	c.Flags().BoolVar(&flags.Auth.AuthFlags.SecureCookies, "cookie-secure", false, "Force sending secure flag on http cookies")
	c.Flags().DurationVar(&flags.Auth.AuthFlags.Expiration, "auth-duration", CmdDefaults.Auth.AuthFlags.Expiration, "Length of time for which tokens are valid. Afterwards, users will have to log back in.")
	c.Flags().Var(flags.Auth.AuthFlags.SigningKey, "session-signing-key", "File containing an RSA private key, used to sign auth tokens.")
	c.Flags().Var(&flags.Auth.AuthFlags.LocalUsers, "add-local-user", "List of username:password combinations for all your local users. The password can be bcrypted - if so, it must have a minimum cost of 10. Ex. USERNAME:PASSWORD")
	c.Flags().Var(&flags.Auth.AuthFlags.Clients, "add-client", "List of client_id:client_secret combinations. Ex. CLIENT_ID:CLIENT_SECRET")
	c.Flags().StringVar(&flags.Auth.AuthFlags.PasswordConnector, "password-connector", CmdDefaults.Auth.AuthFlags.PasswordConnector, "Connector to use when authenticating via 'fly login -u ... -p ...'")

	InitializeConnectorFlags(c, flags)

	// Main Team Flags
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.LocalUsers, "main-team-local-user", nil, "A whitelisted local concourse user. These are the users you've added at web startup with the --add-local-user flag. Ex. USERNAME")
	c.Flags().VarP(&flags.Auth.MainTeamFlags.Config, "main-team-config", "c", "Configuration file for specifying team params")
	InitializeTeamConnectorsFlags(c, flags)
}

func InitializeSystemClaimFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().StringVar(&flags.SystemClaim.Key, "system-claim-key", CmdDefaults.SystemClaim.Key, "The token claim key to use when matching system-claim-values")
	c.Flags().StringSliceVar(&flags.SystemClaim.Values, "system-claim-value", CmdDefaults.SystemClaim.Values, "Configure which token requests should be considered 'system' requests.")
}

func InitializeExperimentalFlags(c *cobra.Command, flags *RunConfig) {
	c.Flags().BoolVar(&flags.FeatureFlags.EnableBuildRerunWhenWorkerDisappears, "enable-rerun-when-worker-disappears", false, "Enable automatically build rerun when worker disappears")
	c.Flags().BoolVar(&flags.FeatureFlags.EnableGlobalResources, "enable-global-resources", false, "Enable equivalent resources across pipelines and teams to share a single version history.")
	c.Flags().BoolVar(&flags.FeatureFlags.EnableRedactSecrets, "enable-redact-secrets", false, "Enable redacting secrets in build logs.")
	c.Flags().BoolVar(&flags.FeatureFlags.EnableAcrossStep, "enable-across-step", false, "Enable the experimental across step to be used in jobs. The API is subject to change.")
	c.Flags().BoolVar(&flags.FeatureFlags.EnablePipelineInstances, "enable-pipeline-instances", false, "Enable pipeline instances")
	c.Flags().BoolVar(&flags.FeatureFlags.EnableP2PVolumeStreaming, "enable-p2p-volume-streaming", false, "Enable P2P volume streaming")
}
