package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
)

func httpClient() *http.Client {
	var client *http.Client

	caCertContents, certProvided := os.LookupEnv("FLY_CA_CERT")
	if certProvided {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(caCertContents))
		tlsConfig := &tls.Config{
			RootCAs: caCertPool,
		}

		transport := &http.Transport{TLSClientConfig: tlsConfig}
		client = &http.Client{Transport: transport}
	} else {
		client = http.DefaultClient
	}

	return client
}
