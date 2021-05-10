package validator

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/concourse/flag"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"gopkg.in/yaml.v2"
)

// NewValidator constructs a new validator that contains all the default and
// custom validations that are necessary to run against the concourse config
// fields. These include validations on the atc, tsa, worker and baggageclaim
// fields.
func NewValidator(trans ut.Translator) *validator.Validate {
	validate := validator.New()
	en_translations.RegisterDefaultTranslations(validate, trans)

	// Register the validations that will be run against any field that is of the
	// type struct that is registered
	validate.RegisterStructValidation(ValidateURL, flag.URL{})
	validate.RegisterStructValidation(ValidateTLSOrLetsEncrypt, atccmd.TLSConfig{})

	// Construct list of all custom validations that will be performed on an
	// individual field that contains that tag value
	validations := map[string]validator.Func{
		"limited_route":       ValidateLimitedRoute,
		"rbac":                ValidateRBAC,
		"cps":                 ValidateContainerPlacementStrategy,
		"sac":                 ValidateStreamingArtifactsCompression,
		"log_level":           ValidateLogLevel,
		"connectors":          ValidateConnectors,
		"ip_version":          baggageclaimcmd.ValidateIPVersion,
		"baggageclaim_driver": baggageclaimcmd.ValidateBaggageclaimDriver,
		"runtime":             ValidateRuntime,
		"creds_manager":       ValidateCredentialManager,
		"metrics_emitter":     ValidateMetricsEmitter,
		"tracing_provider":    ValidateTracingProvider,
	}

	// Loop over each validation and register them with the validator
	for validationTag, validationFunc := range validations {
		validate.RegisterValidation(validationTag, validationFunc)
	}

	// Register all the custom error messages for each validation. Most custom
	// validations have their own error message.
	ve := NewValidatorErrors(validate, trans)
	ve.SetupErrorMessages()

	return validate
}

// All the possible custom error messages for each tag validation
var (
	ValidationErrParseURL          = "url is invalid"
	ValidationErrLimitedRoute      = fmt.Sprintf("Not a valid route to limit. Valid routes include %v.", wrappa.SupportedActions)
	ValidationErrEmptyTLSBindPort  = "must specify tls.bind_port to use TLS"
	ValidationErrEnableLetsEncrypt = "cannot specify lets_encrypt.enable if tls.cert or tls.key are set"
	ValidationErrTLSCertKey        = "must specify HTTPS external-url to use TLS"
	ValidationErrTLS               = "must specify tls.cert and tls.key, or lets_encrypt.enable to use TLS"
	ValidationErrRBAC              = "unknown rbac role or action defined in the config rbac file provided"
	ValidationErrCPS               = fmt.Sprintf("Not a valid list of container placement strategies. Valid strategies include %v.", atc.ValidContainerPlacementStrategies)
	ValidationErrSAC               = fmt.Sprintf("Not a valid streaming artifacts compression. Valid options include %v.", atc.ValidStreamingArtifactsCompressions)
	ValidationErrLogLevel          = fmt.Sprintf("Not a valid log level. Valid options include %v.", flag.ValidLogLevels)
	ValidationErrRuntime           = fmt.Sprintf("Not a valid runtime. Valid options include %v.", workercmd.ValidRuntimes)
)

type ValidationCredsManagerError struct{}

func (e ValidationCredsManagerError) Error() string {
	var credsNames []string
	credsManagers := atccmd.CredentialManagersConfig{}
	for name := range credsManagers.All() {
		credsNames = append(credsNames, name)
	}
	return fmt.Sprintf("Not a valid creds manager. Valid options include %v.", credsNames)
}

type ValidationMetricsEmitterError struct{}

func (e ValidationMetricsEmitterError) Error() string {
	var emitters []string
	metricsEmitters := atccmd.MetricsEmitterConfig{}
	for name := range metricsEmitters.All() {
		emitters = append(emitters, name)
	}
	return fmt.Sprintf("Not a valid metrics emitter. Valid options include %v.", emitters)
}

type validatorErrors struct {
	validate *validator.Validate
	trans    ut.Translator
}

func NewValidatorErrors(validate *validator.Validate, trans ut.Translator) *validatorErrors {
	return &validatorErrors{
		validate: validate,
		trans:    trans,
	}
}

// SetupErrorMessages registers all the custom error messages that will be
// returned to the user when the validation fails
func (v *validatorErrors) SetupErrorMessages() {
	validationErrorMessages := map[string]string{
		"parseurl":      ValidationErrParseURL,
		"limited_route": ValidationErrLimitedRoute,

		"tlsemptybindport":  ValidationErrEmptyTLSBindPort,
		"letsencryptenable": ValidationErrEnableLetsEncrypt,
		"tlsexternalurl":    ValidationErrTLSCertKey,
		"tlsorletsencrypt":  ValidationErrTLS,

		"rbac":      ValidationErrRBAC,
		"cps":       ValidationErrCPS,
		"sac":       ValidationErrSAC,
		"log_level": ValidationErrLogLevel,

		"ip_version":          baggageclaimcmd.ValidationErrIPVersion,
		"baggageclaim_driver": baggageclaimcmd.ValidationErrBaggageclaimDriver,
		"runtime":             ValidationErrRuntime,

		"connectors":       skycmd.ValidationConnectorsError{}.Error(),
		"creds_manager":    ValidationCredsManagerError{}.Error(),
		"metrics_emitter":  ValidationMetricsEmitterError{}.Error(),
		"tracing_provider": tracing.ValidationTracingProviderError{}.Error(),
	}

	for errorTag, errorMessage := range validationErrorMessages {
		v.RegisterTranslation(errorTag, errorMessage)
	}
}

func (v *validatorErrors) RegisterTranslation(validationName string, errorString string) {
	v.validate.RegisterTranslation(validationName, v.trans, func(ut ut.Translator) error {
		return ut.Add(validationName, errorString, true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(validationName, fe.Field())
		return fmt.Sprintf(`error: %s,
value: %s=%s`, t, fe.Field(), fe.Value())
	})
}

func ValidateURL(sl validator.StructLevel) {
	flagURL := sl.Current().Interface().(flag.URL)
	if flagURL.URL == nil {
		return
	}

	value := normalizeURL(flagURL.String())
	parsedURL, err := url.Parse(value)
	if err != nil {
		sl.ReportError(flagURL.String(), "url", sl.Current().Type().Name(), "parseurl", "")
		return
	}

	// localhost URLs that do not start with http:// are interpreted
	// with `localhost` as the Scheme, not the Host
	if parsedURL.Scheme == "" {
		sl.ReportError(flagURL.String(), "url", sl.Current().Type().Name(), "urlscheme", "")
		return
	}

	if parsedURL.Host == "" {
		sl.ReportError(flagURL.String(), "url", sl.Current().Type().Name(), "urlhost", "")
		return
	}
}

func ValidateLimitedRoute(field validator.FieldLevel) bool {
	for _, route := range atc.Routes {
		// Ensure the value exists within the recognized routes
		if route.Name == field.Field().String() {
			for _, supportedAction := range wrappa.SupportedActions {
				// Check if the value is one of the supported actions
				if field.Field().String() == supportedAction {
					return true
				}
			}

		}
	}

	return false
}

func ValidateTLSOrLetsEncrypt(sl validator.StructLevel) {
	var (
		tlsConfig         atccmd.TLSConfig
		letsEncryptConfig atccmd.LetsEncryptConfig
	)

	tlsConfig = sl.Current().Interface().(atccmd.TLSConfig)
	if sl.Top().FieldByName("LetsEncrypt").Interface() != nil {
		letsEncryptConfig = sl.Top().FieldByName("LetsEncrypt").Interface().(atccmd.LetsEncryptConfig)
	}

	switch {
	case tlsConfig.BindPort == 0:
		if tlsConfig.Cert != "" || tlsConfig.Key != "" || letsEncryptConfig.Enable {
			sl.ReportError(tlsConfig.BindPort, "tls.bind_port", sl.Current().Type().Name(), "tlsemptybindport", "")
		}
	case letsEncryptConfig.Enable:
		if tlsConfig.Cert != "" || tlsConfig.Key != "" {
			sl.ReportError(letsEncryptConfig.Enable, "lets_encrypt.enable", sl.Current().Type().Name(), "letsencryptenable", "")
		}
	case tlsConfig.Cert != "" && tlsConfig.Key != "":
		var externalURLField flag.URL
		if sl.Top().FieldByName("ExternalURL").Interface() != nil {
			externalURLField = sl.Top().FieldByName("ExternalURL").Interface().(flag.URL)
		}

		if externalURLField.Scheme != "https" {
			sl.ReportError(externalURLField.String(), "external_url", sl.Current().Type().Name(), "tlsexternalurl", "")
		}
	default:
		sl.ReportError("", "tls.cert or tls.key or lets_encrypt.enable", sl.Current().Type().Name(), "tlsorletsencrypt", "")
	}
}

func ValidateRBAC(field validator.FieldLevel) bool {
	path := field.Field().String()
	if path == "" {
		return true
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var data map[string][]string
	if err = yaml.Unmarshal(content, &data); err != nil {
		return false
	}

	allKnownRoles := map[string]bool{}
	for _, roleName := range accessor.DefaultRoles {
		allKnownRoles[roleName] = true
	}

	for role, actions := range data {
		if _, ok := allKnownRoles[role]; !ok {
			return false
		}

		for _, action := range actions {
			if _, ok := accessor.DefaultRoles[action]; !ok {
				return false
			}
		}
	}

	return true
}

func ValidateContainerPlacementStrategy(field validator.FieldLevel) bool {
	value := field.Field().String()
	for _, validChoice := range atc.ValidContainerPlacementStrategies {
		if value == string(validChoice) {
			return true
		}
	}

	return false
}

func ValidateStreamingArtifactsCompression(field validator.FieldLevel) bool {
	value := field.Field().String()
	for _, validChoice := range atc.ValidStreamingArtifactsCompressions {
		if value == string(validChoice) {
			return true
		}
	}

	return false
}

func ValidateLogLevel(field validator.FieldLevel) bool {
	value := field.Field().String()
	for _, validChoice := range flag.ValidLogLevels {
		if value == string(validChoice) {
			return true
		}
	}

	return false
}

func ValidateConnectors(field validator.FieldLevel) bool {
	userIDPerConnector := field.Field().Interface().(flag.StringToString)

	for connectorId, fieldName := range userIDPerConnector {
		valid := false
		if connectorId == "local" {
			valid = true
		} else {
			teamConnectors := skycmd.TeamConnectorsConfig{}
			for _, connector := range teamConnectors.AllConnectors() {
				if connector.ID() == connectorId {
					valid = true
					break
				}
			}
		}

		if !valid {
			return false
		}

		switch fieldName {
		case "user_id", "name", "username", "email":
		default:
			return false
		}
	}

	return true
}

// Not sure how to test this because it is within the worker_linux file
func ValidateRuntime(field validator.FieldLevel) bool {
	value := field.Field().String()
	for _, validChoice := range workercmd.ValidRuntimes {
		if value == string(validChoice) {
			return true
		}
	}

	return false
}

func ValidateCredentialManager(field validator.FieldLevel) bool {
	value := field.Field().String()
	if value == "" {
		return true
	}

	credsManagers := atccmd.CredentialManagersConfig{}
	for name := range credsManagers.All() {
		if value == name {
			return true
		}
	}

	return false
}

func ValidateMetricsEmitter(field validator.FieldLevel) bool {
	value := field.Field().String()
	if value == "" {
		return true
	}

	metricsEmitters := atccmd.MetricsEmitterConfig{}
	for name := range metricsEmitters.All() {
		if value == name {
			return true
		}
	}

	return false
}

func ValidateTracingProvider(field validator.FieldLevel) bool {
	value := field.Field().String()
	if value == "" {
		return true
	}

	tracingProviders := tracing.ProvidersConfig{}
	for name := range tracingProviders.All() {
		if value == name {
			return true
		}
	}

	return false
}

func normalizeURL(urlIn string) string {
	return strings.TrimRight(urlIn, "/")
}
