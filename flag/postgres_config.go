package flag

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type PostgresConfig struct {
	Host string `yaml:"host"`
	Port uint16 `yaml:"port"`

	Socket string `yaml:"socket"`

	User     string `yaml:"user"`
	Password string `yaml:"password"`

	SSLMode    string `yaml:"sslmode"`
	CACert     File   `yaml:"ca_cert"`
	ClientCert File   `yaml:"client_cert"`
	ClientKey  File   `yaml:"client_key"`

	ConnectTimeout time.Duration `yaml:"connect_timeout"`

	Database string `yaml:"database"`
}

var strEsc = regexp.MustCompile(`([\\'])`)

func (config PostgresConfig) ConnectionString() string {
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
