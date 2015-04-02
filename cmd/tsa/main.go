package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
	"golang.org/x/crypto/ssh"
)

var port = flag.Int(
	"port",
	8800,
	"port to listen for ssh connections on",
)

var hostKeyPath = flag.String(
	"hostKey",
	"",
	"path to private host key",
)

var authorizedKeysPath = flag.String(
	"authorizedKeys",
	"",
	"path to authorized keys",
)

var atcAPIURL = flag.String(
	"atcAPIURL",
	"",
	"ATC API endpoint to register workers with",
)

var heartbeatInterval = flag.Duration(
	"heartbeatInterval",
	30*time.Second,
	"interval on which to heartbeat workers to the ATC",
)

var forwardHost = flag.String(
	"forwardHost",
	"",
	"host on which to listen for forwarding requests",
)

func main() {
	flag.Parse()

	logger := lager.NewLogger("tsa")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	if len(*forwardHost) == 0 {
		logger.Fatal("missing-flag", nil, lager.Data{"flag": "-forwardHost"})
	}

	atcEndpoint := rata.NewRequestGenerator(*atcAPIURL, atc.Routes)

	authorizedKeys, err := loadAuthorizedKeys(*authorizedKeysPath)
	if err != nil {
		logger.Fatal("failed-to-load-authorized-keys", err)
	}

	config, err := configureSSHServer(*hostKeyPath, authorizedKeys)
	if err != nil {
		logger.Fatal("failed-to-configure-ssh-server", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		logger.Fatal("failed-to-listen-for-connection", err)
	}

	logger.Info("listening")

	server := &registrarSSHServer{
		logger:            logger,
		heartbeatInterval: *heartbeatInterval,
		atcEndpoint:       atcEndpoint,
		forwardHost:       *forwardHost,
		config:            config,
		httpClient:        http.DefaultClient,
	}

	server.Serve(listener)
}

func loadAuthorizedKeys(path string) ([]ssh.PublicKey, error) {
	authorizedKeysBytes, err := ioutil.ReadFile(path)
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

func configureSSHServer(hostKeyPath string, authorizedKeys []ssh.PublicKey) (*ssh.ServerConfig, error) {
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

			return nil, fmt.Errorf("unknown public key")
		},
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return certChecker.Authenticate(conn, key)
		},
	}

	privateBytes, err := ioutil.ReadFile(hostKeyPath)
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
