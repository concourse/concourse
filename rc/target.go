package rc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/concourse/fly/ui"
	"github.com/concourse/fly/version"
	"github.com/concourse/go-concourse/concourse"
	"golang.org/x/oauth2"
)

type ErrVersionMismatch struct {
	flyVersion string
	atcVersion string
	targetName TargetName
}

func NewErrVersionMismatch(flyVersion string, atcVersion string, targetName TargetName) ErrVersionMismatch {
	return ErrVersionMismatch{
		flyVersion: flyVersion,
		atcVersion: atcVersion,
		targetName: targetName,
	}
}

func (e ErrVersionMismatch) Error() string {
	return fmt.Sprintf("fly version (%s) is out of sync with the target (%s). to sync up, run the following:\n\n    fly -t %s sync\n", ui.Embolden(e.flyVersion), ui.Embolden(e.atcVersion), e.targetName)
}

type Target interface {
	Client() concourse.Client
	Team() concourse.Team
	Validate() error
	ValidateWithWarningOnly() error
}

type target struct {
	name     TargetName
	teamName string
	client   concourse.Client
}

func LoadTarget(selectedTarget TargetName) (Target, error) {
	targetProps, err := SelectTarget(selectedTarget)
	if err != nil {
		return nil, err
	}

	caCertPool, err := loadCACertPool(targetProps.CACert)
	if err != nil {
		return nil, err
	}

	httpClient := defaultHttpClient(targetProps.Token, targetProps.Insecure, caCertPool)
	client := concourse.NewClient(targetProps.API, httpClient)

	return &target{
		name:     selectedTarget,
		teamName: targetProps.TeamName,
		client:   client,
	}, nil
}

func LoadTargetWithInsecure(
	selectedTarget TargetName,
	teamName string,
	commandInsecure bool,
	caCert string,
) (Target, error) {
	targetProps, err := SelectTarget(selectedTarget)
	if err != nil {
		return nil, err
	}

	if teamName == "" {
		teamName = targetProps.TeamName
	}

	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}

	httpClient := defaultHttpClient(targetProps.Token, commandInsecure, caCertPool)

	return &target{
		name:     selectedTarget,
		teamName: teamName,
		client:   concourse.NewClient(targetProps.API, httpClient),
	}, nil
}

func NewUnauthenticatedTarget(
	name TargetName,
	url string,
	teamName string,
	insecure bool,
	caCert string,
) (Target, error) {
	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}

	httpClient := unauthenticatedHttpClient(insecure, caCertPool)
	client := concourse.NewClient(url, httpClient)
	return &target{
		name:     name,
		teamName: teamName,
		client:   client,
	}, nil
}

func NewBasicAuthTarget(
	name TargetName,
	url string,
	teamName string,
	insecure bool,
	username string,
	password string,
	caCert string,
) (Target, error) {
	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}
	httpClient := basicAuthHttpClient(username, password, insecure, caCertPool)
	client := concourse.NewClient(url, httpClient)

	return &target{
		name:     name,
		teamName: teamName,
		client:   client,
	}, nil
}

func NewNoAuthTarget(
	name TargetName,
	url string,
	teamName string,
	insecure bool,
	caCert string,
) (Target, error) {
	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: transport(insecure, caCertPool)}
	client := concourse.NewClient(url, httpClient)

	return &target{
		name:     name,
		teamName: teamName,
		client:   client,
	}, nil
}

func (t *target) Client() concourse.Client {
	return t.client
}

func (t *target) Team() concourse.Team {
	return t.client.Team(t.teamName)
}

func (t *target) ValidateWithWarningOnly() error {
	return t.validate(true)
}

func (t *target) Validate() error {
	return t.validate(false)
}

func (t *target) validate(allowVersionMismatch bool) error {
	info, err := t.client.GetInfo()
	if err != nil {
		return err
	}

	if info.Version == version.Version || version.IsDev(version.Version) {
		return nil
	}

	atcMajor, atcMinor, atcPatch, err := version.GetSemver(info.Version)
	if err != nil {
		return err
	}

	flyMajor, flyMinor, flyPatch, err := version.GetSemver(version.Version)
	if err != nil {
		return err
	}

	if !allowVersionMismatch && (atcMajor != flyMajor || atcMinor != flyMinor) {
		return NewErrVersionMismatch(version.Version, info.Version, t.name)
	}

	if atcMajor != flyMajor || atcMinor != flyMinor || atcPatch != flyPatch {
		fmt.Fprintln(os.Stderr, ui.WarningColor("WARNING:\n"))
		fmt.Fprintln(os.Stderr, ui.WarningColor(NewErrVersionMismatch(version.Version, info.Version, t.name).Error()))
	}

	return nil
}

func unauthenticatedHttpClient(insecure bool, caCertPool *x509.CertPool) *http.Client {
	return &http.Client{
		Transport: transport(insecure, caCertPool),
	}
}

func defaultHttpClient(token *TargetToken, insecure bool, caCertPool *x509.CertPool) *http.Client {
	var oAuthToken *oauth2.Token
	if token != nil {
		oAuthToken = &oauth2.Token{
			TokenType:   token.Type,
			AccessToken: token.Value,
		}
	}

	transport := transport(insecure, caCertPool)

	if token != nil {
		transport = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(oAuthToken),
			Base:   transport,
		}
	}

	return &http.Client{Transport: transport}
}

func loadCACertPool(caCert string) (cert *x509.CertPool, err error) {
	if caCert == "" {
		return nil, nil
	}

	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM([]byte(caCert))
	if !ok {
		return nil, errors.New("CA Cert not valid")
	}
	return pool, nil
}

func basicAuthHttpClient(
	username string,
	password string,
	insecure bool,
	caCertPool *x509.CertPool,
) *http.Client {
	return &http.Client{
		Transport: basicAuthTransport{
			username: username,
			password: password,
			base:     transport(insecure, caCertPool),
		},
	}
}

func transport(insecure bool, caCertPool *x509.CertPool) http.RoundTripper {
	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
			RootCAs:            caCertPool,
		},
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		Proxy: http.ProxyFromEnvironment,
	}

	return transport
}

type basicAuthTransport struct {
	username string
	password string

	base http.RoundTripper
}

func (t basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(r)
}
