package atccmd_test

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/flag"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestPlanner(t *testing.T) {
	suite.Run(t, &ValidatorTestSuite{
		Assertions: require.New(t),
	})
}

type ValidatorTestSuite struct {
	trans transHelper

	suite.Suite
	*require.Assertions
}

func (v *ValidatorTestSuite) TestValidatorSuite() {
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")
	transHelper := transHelper{trans}

	for _, test := range UrlTests {
		v.Run(test.Title, func() {
			test.TestURLValidator(v, transHelper)
		})
	}

	for _, test := range LimitedRouteTests {
		v.Run(test.Title, func() {
			test.TestLimitedRouteValidator(v, transHelper)
		})
	}

	for _, test := range TLSOrLetsEncryptTests {
		v.Run(test.Title, func() {
			test.TestTLSOrLetsEncryptValidator(v, transHelper)
		})
	}

	for _, test := range RBACTests {
		v.Run(test.Title, func() {
			test.TestRBACValidator(v, transHelper)
		})
	}

	for _, test := range ContainerPlacementStrategyTests {
		v.Run(test.Title, func() {
			test.TestContainerPlacementStrategyValidator(v, transHelper)
		})
	}

	for _, test := range StreamingArtifactsCompressionTests {
		v.Run(test.Title, func() {
			test.TestStreamingArtifactsCompressionValidator(v, transHelper)
		})
	}
}

type transHelper struct {
	trans ut.Translator
}

func (t transHelper) RegisterTranslation(validate *validator.Validate, validationName string, errorString string) {
	validate.RegisterTranslation(validationName, t.trans, func(ut ut.Translator) error {
		return ut.Add(validationName, errorString, true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(validationName, fe.Field())
		return fmt.Sprintf(`error: %s,
value: %s=%s`, t, fe.Field(), fe.Value())
	})
}

type URLTest struct {
	Title string
	URL   string
	Valid bool
}

var UrlTests = []URLTest{
	{
		Title: "simple url",
		URL:   "https://localhost:8080",
		Valid: true,
	},
	{
		Title: "url with trimmed path",
		URL:   "http://localhost/foo",
		Valid: true,
	},
	{
		Title: "url without scheme",
		URL:   "localhost:8080",
		Valid: false,
	},
	{
		Title: "url without host",
		URL:   "foo/bar",
		Valid: false,
	},
}

func (t *URLTest) TestURLValidator(s *ValidatorTestSuite, trans transHelper) {
	parsedUrl, err := url.Parse(t.URL)
	s.NoError(err)

	testStruct := struct {
		URL flag.URL
	}{
		URL: flag.URL{parsedUrl},
	}

	validate := validator.New()
	validate.RegisterStructValidation(atccmd.ValidateURL, flag.URL{})

	err = validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Assert().Error(err)
	}
}

type LimitedRouteTest struct {
	Title string
	Route string
	Valid bool
}

var LimitedRouteTests = []LimitedRouteTest{
	{
		Title: "valid limited route",
		Route: "ListAllJobs",
		Valid: true,
	},
	{
		Title: "non existant route",
		Route: "Woopie",
		Valid: false,
	},
	{
		Title: "non supported action",
		Route: "CheckResource",
		Valid: false,
	},
}

func (t *LimitedRouteTest) TestLimitedRouteValidator(s *ValidatorTestSuite, trans transHelper) {
	testStruct := struct {
		Route string `validate:"limited_route"`
	}{
		Route: t.Route,
	}

	validate := validator.New()
	validate.RegisterValidation("limited_route", atccmd.ValidateLimitedRoute)
	trans.RegisterTranslation(validate, "limited_route", atccmd.ValidationErrLimitedRoute)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), atccmd.ValidationErrLimitedRoute)
	}
}

type TLSOrLetsEncryptTest struct {
	Title string
	Valid bool

	TLS         atccmd.TLSConfig
	LetsEncrypt atccmd.LetsEncryptConfig
	ExternalURL string

	ErrorMessage string
}

var TLSOrLetsEncryptTests = []TLSOrLetsEncryptTest{
	{
		Title: "valid empty tls bind port",

		TLS: atccmd.TLSConfig{
			BindPort: 0,
		},

		Valid: true,
	},
	{
		Title: "empty tls bind port with cert configured",

		TLS: atccmd.TLSConfig{
			Cert: flag.File("path/cert"),
		},

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrEmptyTLSBindPort,
	},
	{
		Title: "empty tls bind port with key configured",

		TLS: atccmd.TLSConfig{
			Key: flag.File("path/key"),
		},

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrEmptyTLSBindPort,
	},
	{
		Title: "empty tls bind port with lets encrypt enabled",

		LetsEncrypt: atccmd.LetsEncryptConfig{
			Enable: true,
		},

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrEmptyTLSBindPort,
	},
	{
		Title: "lets encrypt enabled with no tls cert or key configured",

		TLS: atccmd.TLSConfig{
			BindPort: 1234,
		},
		LetsEncrypt: atccmd.LetsEncryptConfig{
			Enable: true,
		},

		Valid: true,
	},
	{
		Title: "lets encrypt enabled with tls cert configured",

		TLS: atccmd.TLSConfig{
			BindPort: 1234,
			Cert:     flag.File("path/cert"),
		},
		LetsEncrypt: atccmd.LetsEncryptConfig{
			Enable: true,
		},

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrEnableLetsEncrypt,
	},
	{
		Title: "lets encrypt enabled with tls key configured",

		TLS: atccmd.TLSConfig{
			BindPort: 1234,
			Key:      flag.File("path/key"),
		},
		LetsEncrypt: atccmd.LetsEncryptConfig{
			Enable: true,
		},

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrEnableLetsEncrypt,
	},
	{
		Title: "tls cert and key configured with valid https external url",

		TLS: atccmd.TLSConfig{
			BindPort: 1234,
			Cert:     flag.File("path/cert"),
			Key:      flag.File("path/key"),
		},
		ExternalURL: "https://localhost",

		Valid: true,
	},
	{
		Title: "tls cert and key configured without external url",

		TLS: atccmd.TLSConfig{
			BindPort: 1234,
			Cert:     flag.File("path/cert"),
			Key:      flag.File("path/key"),
		},

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrTLSCertKey,
	},
	{
		Title: "tls cert and key configured without https within external url",

		TLS: atccmd.TLSConfig{
			BindPort: 1234,
			Cert:     flag.File("path/cert"),
			Key:      flag.File("path/key"),
		},
		ExternalURL: "http://localhost",

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrTLSCertKey,
	},
	{
		Title: "neither tls or lets encrypt enabled",

		TLS: atccmd.TLSConfig{
			BindPort: 1234,
		},

		Valid:        false,
		ErrorMessage: atccmd.ValidationErrTLS,
	},
}

func (t *TLSOrLetsEncryptTest) TestTLSOrLetsEncryptValidator(s *ValidatorTestSuite, trans transHelper) {
	parsedURL, _ := url.Parse(t.ExternalURL)

	testStruct := struct {
		TLS         atccmd.TLSConfig
		ExternalURL flag.URL
		LetsEncrypt atccmd.LetsEncryptConfig
	}{
		TLS:         t.TLS,
		ExternalURL: flag.URL{parsedURL},
		LetsEncrypt: t.LetsEncrypt,
	}

	validate := validator.New()
	validate.RegisterStructValidation(atccmd.ValidateTLSOrLetsEncrypt, atccmd.TLSConfig{})

	trans.RegisterTranslation(validate, "tlsemptybindport", atccmd.ValidationErrEmptyTLSBindPort)
	trans.RegisterTranslation(validate, "letsencryptenable", atccmd.ValidationErrEnableLetsEncrypt)
	trans.RegisterTranslation(validate, "tlsexternalurl", atccmd.ValidationErrTLSCertKey)
	trans.RegisterTranslation(validate, "tlsorletsencrypt", atccmd.ValidationErrTLS)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), t.ErrorMessage)
	}
}

type RBACTest struct {
	Title       string
	ConfigRBAC  string
	Valid       bool
	UnknownFile bool
}

var RBACTests = []RBACTest{
	{
		Title: "rbac valid config",
		ConfigRBAC: `member:
  - AbortBuild`,
		Valid: true,
	},
	{
		Title: "rbac unknown role",
		ConfigRBAC: `unknown_role:
  - AbortBuild`,
		Valid: false,
	},
	{
		Title: "rbac unknown action",
		ConfigRBAC: `member:
  - UnknownAction`,
		Valid: false,
	},
	{
		Title: "rbac unmarshalable file into data structure",
		ConfigRBAC: `foo:
  bar:
	  file: true`,
		Valid: false,
	},
	{
		Title:       "rbac config file not found",
		UnknownFile: true,
		Valid:       false,
	},
}

func (t *RBACTest) TestRBACValidator(s *ValidatorTestSuite, trans transHelper) {
	path := "some/path"
	if !t.UnknownFile {
		path = filepath.Join(s.T().TempDir(), "data.yml")
		err := os.WriteFile(path, []byte(t.ConfigRBAC), os.ModePerm)
		s.Assert().NoError(err)
	}

	testStruct := struct {
		ConfigRBAC flag.File `validate:"rbac"`
	}{
		ConfigRBAC: flag.File(path),
	}

	validate := validator.New()
	validate.RegisterValidation("rbac", atccmd.ValidateRBAC)
	trans.RegisterTranslation(validate, "rbac", atccmd.ValidationErrRBAC)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), atccmd.ValidationErrRBAC)
	}
}

type ContainerPlacementStrategyTest struct {
	Title                      string
	ContainerPlacementStrategy []string
	Valid                      bool
}

var ContainerPlacementStrategyTests = []ContainerPlacementStrategyTest{
	{
		Title:                      "cps valid container placement strategy",
		ContainerPlacementStrategy: []string{"random"},
		Valid:                      true,
	},
	{
		Title:                      "cps invalid container placement strategy",
		ContainerPlacementStrategy: []string{"invalid-strategy"},
		Valid:                      false,
	},
	{
		Title:                      "cps list of container placement strategies",
		ContainerPlacementStrategy: []string{"volume-locality", "fewest-build-containers", "limit-active-tasks"},
		Valid:                      true,
	},
	{
		Title:                      "cps list of container placement strategies with one invalid",
		ContainerPlacementStrategy: []string{"volume-locality", "fewest-build-containers", "invalid-strategy"},
		Valid:                      false,
	},
}

func (t *ContainerPlacementStrategyTest) TestContainerPlacementStrategyValidator(s *ValidatorTestSuite, trans transHelper) {
	testStruct := struct {
		ContainerPlacementStrategy []string `validate:"dive,cps"`
	}{
		ContainerPlacementStrategy: t.ContainerPlacementStrategy,
	}

	validate := validator.New()
	validate.RegisterValidation("cps", atccmd.ValidateContainerPlacementStrategy)
	trans.RegisterTranslation(validate, "cps", atccmd.ValidationErrCPS)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), atccmd.ValidationErrCPS)
	}
}

type StreamingArtifactsCompressionTest struct {
	Title                         string
	StreamingArtifactsCompression string
	Valid                         bool
}

var StreamingArtifactsCompressionTests = []StreamingArtifactsCompressionTest{
	{
		Title:                         "sac valid streaming artifacts compression",
		StreamingArtifactsCompression: "gzip",
		Valid:                         true,
	},
	{
		Title:                         "sac invalid streaming artifacts compression",
		StreamingArtifactsCompression: "invalid",
		Valid:                         false,
	},
}

func (t *StreamingArtifactsCompressionTest) TestStreamingArtifactsCompressionValidator(s *ValidatorTestSuite, trans transHelper) {
	testStruct := struct {
		StreamingArtifactsCompression string `validate:"sac"`
	}{
		StreamingArtifactsCompression: t.StreamingArtifactsCompression,
	}

	validate := validator.New()
	validate.RegisterValidation("sac", atccmd.ValidateStreamingArtifactsCompression)
	trans.RegisterTranslation(validate, "sac", atccmd.ValidationErrSAC)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), atccmd.ValidationErrSAC)
	}
}

type LogLevelsTest struct {
	Title    string
	LogLevel string
	Valid    bool
}

var LogLevelsTests = []LogLevelsTest{
	{
		Title:    "log level valid choice",
		LogLevel: "debug",
		Valid:    true,
	},
	{
		Title:    "log level invalid choice",
		LogLevel: "invalid-log-level",
		Valid:    false,
	},
}

func (t *LogLevelsTest) TestLogLevelValidator(s *ValidatorTestSuite, trans transHelper) {
	testStruct := struct {
		LogLevel string `validate:"log_level"`
	}{
		LogLevel: t.LogLevel,
	}

	validate := validator.New()
	validate.RegisterValidation("log_level", atccmd.ValidateLogLevel)
	trans.RegisterTranslation(validate, "log_level", atccmd.ValidationErrLogLevel)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), atccmd.ValidationErrLogLevel)
	}
}
