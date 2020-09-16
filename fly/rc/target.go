package rc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	conc "github.com/concourse/concourse"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/fly/version"
	"github.com/concourse/concourse/go-concourse/concourse"
	semisemanticversion "github.com/cppforlife/go-semi-semantic/version"
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
	return fmt.Sprintf(
		"fly version (%s) is out of sync with the target (%s). to sync up, run the following:\n\n    %s -t %s sync\n",
		ui.Embolden(e.flyVersion), ui.Embolden(e.atcVersion), os.Args[0], e.targetName)
}

//go:generate counterfeiter . Target

type Target interface {
	Client() concourse.Client
	Team() concourse.Team
	FindTeam(string) (concourse.Team, error)
	CACert() string
	ClientCertPath() string
	ClientKeyPath() string
	ClientCertificate() []tls.Certificate
	Validate() error
	ValidateWithWarningOnly() error
	TLSConfig() *tls.Config
	URL() string
	WorkerVersion() (string, error)
	IsWorkerVersionCompatible(string) (bool, error)
	Token() *TargetToken
	TokenAuthorization() (string, bool)
	Version() (string, error)
}

type target struct {
	name              TargetName
	teamName          string
	caCert            string
	clientCertPath    string
	clientKeyPath     string
	clientCertificate []tls.Certificate
	tlsConfig         *tls.Config
	client            concourse.Client
	url               string
	token             *TargetToken
	info              atc.Info
}

func NewTarget(
	name TargetName,
	teamName string,
	url string,
	token *TargetToken,
	caCert string,
	caCertPool *x509.CertPool,
	clientCertPath string,
	clientKeyPath string,
	clientCertificate []tls.Certificate,
	insecure bool,
	client concourse.Client,
) *target {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
		RootCAs:            caCertPool,
		Certificates:       clientCertificate,
	}

	return &target{
		name:              name,
		teamName:          teamName,
		url:               url,
		token:             token,
		caCert:            caCert,
		clientCertPath:    clientCertPath,
		clientKeyPath:     clientKeyPath,
		clientCertificate: clientCertificate,
		tlsConfig:         tlsConfig,
		client:            client,
	}
}

func LoadTargetFromURL(url, team string, tracing bool) (Target, TargetName, error) {
	flyTargets, err := LoadTargets()
	if err != nil {
		return nil, "", err
	}

	for name, props := range flyTargets {
		if props.API == url && props.TeamName == team {
			target, err := LoadTarget(name, tracing)
			return target, name, err
		}
	}

	return nil, "", ErrNoTargetFromURL
}

func LoadTarget(selectedTarget TargetName, tracing bool) (Target, error) {
	var clientCertificate []tls.Certificate

	targetProps, err := selectTarget(selectedTarget)
	if err != nil {
		return nil, err
	}

	caCertPool, err := loadCACertPool(targetProps.CACert)
	if err != nil {
		return nil, err
	}

	clientCertificate, err = loadClientCertificate(targetProps.ClientCertPath, targetProps.ClientKeyPath)
	if err != nil {
		return nil, err
	}

	httpClient := defaultHttpClient(targetProps.Token, targetProps.Insecure, caCertPool, clientCertificate)
	client := concourse.NewClient(targetProps.API, httpClient, tracing)

	return NewTarget(
		selectedTarget,
		targetProps.TeamName,
		targetProps.API,
		targetProps.Token,
		targetProps.CACert,
		caCertPool,
		targetProps.ClientCertPath,
		targetProps.ClientKeyPath,
		clientCertificate,
		targetProps.Insecure,
		client,
	), nil
}

func LoadUnauthenticatedTarget(
	selectedTarget TargetName,
	teamName string,
	insecure bool,
	caCert string,
	clientCertPath string,
	clientKeyPath string,
	tracing bool,
) (Target, error) {
	targetProps, err := selectTarget(selectedTarget)
	if err != nil {
		return nil, err
	}

	if teamName == "" {
		teamName = targetProps.TeamName
	}

	if caCert == "" {
		caCert = targetProps.CACert
	}

	if insecure {
		caCert = ""
	}

	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}

	var clientCertificate []tls.Certificate

	if clientCertPath == "" && clientKeyPath == "" {
		clientCertPath = targetProps.ClientCertPath
		clientKeyPath = targetProps.ClientKeyPath
	}

	clientCertificate, err = loadClientCertificate(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: transport(insecure, caCertPool, clientCertificate)}

	return NewTarget(
		selectedTarget,
		teamName,
		targetProps.API,
		targetProps.Token,
		caCert,
		caCertPool,
		clientCertPath,
		clientKeyPath,
		clientCertificate,
		targetProps.Insecure,
		concourse.NewClient(targetProps.API, httpClient, tracing),
	), nil
}

func NewUnauthenticatedTarget(
	name TargetName,
	url string,
	teamName string,
	insecure bool,
	caCert string,
	clientCertPath string,
	clientKeyPath string,
	tracing bool,
) (Target, error) {
	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}

	var clientCertificate []tls.Certificate
	clientCertificate, err = loadClientCertificate(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: transport(insecure, caCertPool, clientCertificate)}
	client := concourse.NewClient(url, httpClient, tracing)
	return NewTarget(
		name,
		teamName,
		url,
		nil,
		caCert,
		caCertPool,
		clientCertPath,
		clientKeyPath,
		clientCertificate,
		insecure,
		client,
	), nil
}

func NewAuthenticatedTarget(
	name TargetName,
	url string,
	teamName string,
	insecure bool,
	token *TargetToken,
	caCert string,
	clientCertPath string,
	clientKeyPath string,
	tracing bool,
) (Target, error) {
	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}

	var clientCertificate []tls.Certificate
	clientCertificate, err = loadClientCertificate(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, err
	}

	httpClient := defaultHttpClient(token, insecure, caCertPool, clientCertificate)
	client := concourse.NewClient(url, httpClient, tracing)

	return NewTarget(
		name,
		teamName,
		url,
		token,
		caCert,
		caCertPool,
		clientCertPath,
		clientKeyPath,
		clientCertificate,
		insecure,
		client,
	), nil
}

func NewBasicAuthTarget(
	name TargetName,
	url string,
	teamName string,
	insecure bool,
	username string,
	password string,
	caCert string,
	clientCertPath string,
	clientKeyPath string,
	tracing bool,
) (Target, error) {
	caCertPool, err := loadCACertPool(caCert)
	if err != nil {
		return nil, err
	}

	var clientCertificate []tls.Certificate
	clientCertificate, err = loadClientCertificate(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, err
	}

	httpClient := basicAuthHttpClient(username, password, insecure, caCertPool, clientCertificate)
	client := concourse.NewClient(url, httpClient, tracing)

	return NewTarget(
		name,
		teamName,
		url,
		nil,
		caCert,
		caCertPool,
		clientCertPath,
		clientKeyPath,
		clientCertificate,
		insecure,
		client,
	), nil
}

func (t *target) Client() concourse.Client {
	return t.client
}

func (t *target) Team() concourse.Team {
	return t.client.Team(t.teamName)
}

func (t *target) FindTeam(teamName string) (concourse.Team, error) {
	return t.client.FindTeam(teamName)
}

func (t *target) CACert() string {
	return t.caCert
}

func (t *target) TLSConfig() *tls.Config {
	return t.tlsConfig
}

func (t *target) ClientCertPath() string {
	return t.clientCertPath
}

func (t *target) ClientKeyPath() string {
	return t.clientKeyPath
}

func (t *target) ClientCertificate() []tls.Certificate {
	return t.clientCertificate
}

func (t *target) URL() string {
	return t.url
}

func (t *target) Token() *TargetToken {
	return t.token
}

func (t *target) Version() (string, error) {
	info, err := t.getInfo()
	if err != nil {
		return "", err
	}

	return info.Version, nil
}

func (t *target) WorkerVersion() (string, error) {
	info, err := t.getInfo()
	if err != nil {
		return "", err
	}

	return info.WorkerVersion, nil
}

func (t *target) TokenAuthorization() (string, bool) {
	if t.token == nil || (t.token.Type == "" && t.token.Value == "") {
		return "", false
	}

	return t.token.Type + " " + t.token.Value, true
}

func (t *target) ValidateWithWarningOnly() error {
	return t.validate(true)
}

func (t *target) Validate() error {
	return t.validate(false)
}

func (t *target) IsWorkerVersionCompatible(workerVersion string) (bool, error) {
	info, err := t.getInfo()
	if err != nil {
		return false, err
	}

	if info.WorkerVersion == "" {
		return true, nil
	}

	if workerVersion == "" {
		return false, nil
	}

	workerV, err := semisemanticversion.NewVersionFromString(workerVersion)
	if err != nil {
		return false, err
	}

	infoV, err := semisemanticversion.NewVersionFromString(info.WorkerVersion)
	if err != nil {
		return false, err
	}

	if workerV.Release.Components[0].Compare(infoV.Release.Components[0]) != 0 {
		return false, nil
	}

	if workerV.Release.Components[1].Compare(infoV.Release.Components[1]) == -1 {
		return false, nil
	}

	return true, nil
}

func (t *target) validate(allowVersionMismatch bool) error {
	info, err := t.getInfo()
	if err != nil {
		return err
	}

	if info.Version == conc.Version || version.IsDev(conc.Version) {
		return nil
	}

	atcMajor, atcMinor, atcPatch, err := version.GetSemver(info.Version)
	if err != nil {
		return err
	}

	flyMajor, flyMinor, flyPatch, err := version.GetSemver(conc.Version)
	if err != nil {
		return err
	}

	if !allowVersionMismatch && (atcMajor != flyMajor || atcMinor != flyMinor) {
		return NewErrVersionMismatch(conc.Version, info.Version, t.name)
	}

	if atcMajor != flyMajor || atcMinor != flyMinor || atcPatch != flyPatch {
		fmt.Fprintln(ui.Stderr, ui.WarningColor("WARNING:\n"))
		fmt.Fprintln(ui.Stderr, ui.WarningColor(NewErrVersionMismatch(conc.Version, info.Version, t.name).Error()))
	}

	return nil
}

func (t *target) getInfo() (atc.Info, error) {
	if (t.info != atc.Info{}) {
		return t.info, nil
	}

	var err error
	t.info, err = t.client.GetInfo()
	return t.info, err
}

func defaultHttpClient(token *TargetToken, insecure bool, caCertPool *x509.CertPool, clientCertificate []tls.Certificate) *http.Client {
	var oAuthToken *oauth2.Token
	if token != nil {
		oAuthToken = &oauth2.Token{
			TokenType:   token.Type,
			AccessToken: token.Value,
		}
	}

	transport := transport(insecure, caCertPool, clientCertificate)

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

	// TODO: remove else block once we switch to go 1.8
	// x509.SystemCertPool is not supported in go 1.7 on Windows
	// see: https://github.com/golang/go/issues/16736
	var pool *x509.CertPool
	if runtime.GOOS != "windows" {
		var err error
		pool, err = x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
	} else {
		pool = x509.NewCertPool()
	}

	ok := pool.AppendCertsFromPEM([]byte(caCert))
	if !ok {
		return nil, errors.New("CA Cert not valid")
	}
	return pool, nil
}

func loadClientCertificate(clientCertificateLocation string, clientKeyLocation string) (cert []tls.Certificate, err error) {
	if clientCertificateLocation == "" {
		if clientKeyLocation != "" {
			err = errors.New("A client key may not be declared without defining a client certificate")

			return []tls.Certificate{}, err
		}

		return []tls.Certificate{}, nil
	}

	if clientCertificateLocation != "" && clientKeyLocation == "" {
		err = errors.New("A client certificate may not be declared without defining a client key")

		return []tls.Certificate{}, err
	}

	clientCertData, err := tls.LoadX509KeyPair(clientCertificateLocation, clientKeyLocation)
	if err != nil {
		return []tls.Certificate{}, err
	}

	cert = []tls.Certificate{clientCertData}

	return cert, nil
}

func basicAuthHttpClient(
	username string,
	password string,
	insecure bool,
	caCertPool *x509.CertPool,
	clientCertificate []tls.Certificate,
) *http.Client {
	return &http.Client{
		Transport: basicAuthTransport{
			username: username,
			password: password,
			base:     transport(insecure, caCertPool, clientCertificate),
		},
	}
}

func transport(insecure bool, caCertPool *x509.CertPool, clientCertificate []tls.Certificate) http.RoundTripper {
	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
			RootCAs:            caCertPool,
			Certificates:       clientCertificate,
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
