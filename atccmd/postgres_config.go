package atccmd

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type PostgresConfig struct {
	DataSource string `long:"data-source" description:"PostgreSQL connection string. (Deprecated; set the following flags instead.)"`

	Host string `long:"host" description:"The host to connect to." default:"127.0.0.1"`
	Port uint16 `long:"port" description:"The port to connect to." default:"5432"`

	Socket string `long:"socket" description:"Path to a UNIX domain socket to connect to."`

	User     string `long:"user"     description:"The user to sign in as."`
	Password string `long:"password" description:"The user's password."`

	SSLMode    string   `long:"sslmode"     description:"Whether or not to use SSL." default:"disable" choice:"disable" choice:"require" choice:"verify-ca" choice:"verify-full"`
	CACert     FileFlag `long:"ca-cert"     description:"CA cert file location, to verify when connecting with SSL."`
	ClientCert FileFlag `long:"client-cert" description:"Client cert file location."`
	ClientKey  FileFlag `long:"client-key"  description:"Client key file location."`

	ConnectTimeout time.Duration `long:"connect-timeout" description:"Dialing timeout. (0 means wait indefinitely)" default:"5m"`

	Database string `long:"database" description:"The name of the database to use." default:"atc"`
}

func (config PostgresConfig) ConnectionString() string {
	if config.DataSource != "" {
		return config.DataSource
	}

	properties := map[string]interface{}{
		"dbname":  config.Database,
		"sslmode": config.SSLMode,
	}

	if config.User != "" {
		properties["user"] = config.User
	}

	if config.Password != "" {
		properties["password"] = config.Password
	}

	if config.Socket != "" {
		properties["host"] = config.Socket
	} else {
		properties["host"] = config.Host
		properties["port"] = config.Port
	}

	if config.CACert != "" {
		properties["sslrootcert"] = config.CACert.Path()
	}

	if config.ClientCert != "" {
		properties["sslcert"] = config.ClientCert.Path()
	}

	if config.ClientKey != "" {
		properties["sslkey"] = config.ClientKey.Path()
	}

	if config.ConnectTimeout != 0 {
		properties["connect_timeout"] = strconv.Itoa(int(config.ConnectTimeout.Seconds()))
	}

	var pairs []string
	for k, v := range properties {
		var escV string
		switch x := v.(type) {
		case string:
			escV = fmt.Sprintf("'%s'", strEsc.ReplaceAllString(x, `\$1`))
		case uint16:
			escV = fmt.Sprintf("%d", x)
		default:
			panic(fmt.Sprintf("handle %T please", v))
		}

		pairs = append(
			pairs,
			fmt.Sprintf("%s=%s", k, escV),
		)
	}

	sort.Strings(pairs)

	return strings.Join(pairs, " ")
}

var strEsc = regexp.MustCompile(`([\\'])`)
