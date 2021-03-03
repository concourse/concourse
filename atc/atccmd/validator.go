package atccmd

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/flag"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"gopkg.in/yaml.v2"
)

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
)

func NewValidator(trans ut.Translator) *validator.Validate {
	validate := validator.New()
	en_translations.RegisterDefaultTranslations(validate, trans)

	validate.RegisterStructValidation(ValidateURL, flag.URL{})
	validate.RegisterStructValidation(ValidateTLSOrLetsEncrypt, TLSConfig{})
	validate.RegisterValidation("limited_route", ValidateLimitedRoute)
	validate.RegisterValidation("rbac", ValidateRBAC)
	validate.RegisterValidation("cps", ValidateContainerPlacementStrategy)
	validate.RegisterValidation("sac", ValidateStreamingArtifactsCompression)
	validate.RegisterValidation("log_level", ValidateLogLevel)

	ve := NewValidatorErrors(validate, trans)
	ve.SetupErrorMessages()

	return validate
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

func (v *validatorErrors) SetupErrorMessages() {
	v.RegisterTranslation("parseurl", ValidationErrParseURL)
	v.RegisterTranslation("limited_route", ValidationErrLimitedRoute)
	v.RegisterTranslation("tlsemptybindport", ValidationErrEmptyTLSBindPort)
	v.RegisterTranslation("letsencryptenable", ValidationErrEnableLetsEncrypt)
	v.RegisterTranslation("tlsexternalurl", ValidationErrTLSCertKey)
	v.RegisterTranslation("tlsorletsencrypt", ValidationErrTLS)
	v.RegisterTranslation("rbac", ValidationErrRBAC)
	v.RegisterTranslation("cps", ValidationErrCPS)
	v.RegisterTranslation("sac", ValidationErrSAC)
	v.RegisterTranslation("log_level", ValidationErrLogLevel)
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

func ValidateRequired(field validator.FieldLevel) bool {
	parsedIP := net.ParseIP(field.Field().String())
	return parsedIP != nil
}

func ValidateURL(sl validator.StructLevel) {
	flagURL := sl.Current().Interface().(flag.URL)
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
		tlsConfig         TLSConfig
		letsEncryptConfig LetsEncryptConfig
	)

	tlsConfig = sl.Current().Interface().(TLSConfig)
	if sl.Top().FieldByName("LetsEncrypt").Interface() != nil {
		letsEncryptConfig = sl.Top().FieldByName("LetsEncrypt").Interface().(LetsEncryptConfig)
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

func normalizeURL(urlIn string) string {
	return strings.TrimRight(urlIn, "/")
}
