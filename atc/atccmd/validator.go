package atccmd

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/wrappa"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"gopkg.in/yaml.v2"
)

func NewValidator(trans ut.Translator) *validator.Validate {
	validate := validator.New()

	en_translations.RegisterDefaultTranslations(validate, trans)

	// XXX: Can we have better error messages for these?
	validate.RegisterValidation("ip", ValidateIP)
	validate.RegisterValidation("url", ValidateURL)
	validate.RegisterValidation("limited_route", ValidateLimitedRoute)
	validate.RegisterValidation("empty_tls_bind_port", ValidateEmptyTLSBindPort)
	validate.RegisterValidation("enable_lets_encrypt", ValidateEnabledLetsEncrypt)
	validate.RegisterValidation("tls_cert_key", ValidateTLSCertKey)
	validate.RegisterValidation("tls", ValidateTLS)
	validate.RegisterValidation("rbac", ValidateRBAC)
	validate.RegisterValidation("cps", ValidateContainerPlacementStrategy)
	validate.RegisterValidation("sac", ValidateStreamingArtifactsCompression)

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
	// TODO: REGISTER ERROR MESSAGE FO EACH TAG
	// XXX: TEST THIS
	v.RegisterTranslation("limited_route", fmt.Sprintf("Not a valid route to limit. Valid routes include %v.", wrappa.SupportedActions))
	v.RegisterTranslation("empty_tls_bind_port", "must specify tls.bind_port to use TLS")
	v.RegisterTranslation("enable_lets_encrypt", "cannot specify lets_encrypt.enable if tls.cert or tls.key are set")
	v.RegisterTranslation("tls_cert_key", "must specify HTTPS external-url to use TLS")
	v.RegisterTranslation("tls", "must specify tls.cert and tls.key, or lets_encrypt.enable to use TLS")
	v.RegisterTranslation("rbac", "unknown rbac role or action defined in the config rbac file provided")
}

func (v *validatorErrors) RegisterTranslation(validationName string, errorString string) {
	v.validate.RegisterTranslation(validationName, v.trans, func(ut ut.Translator) error {
		return ut.Add(validationName, errorString, true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(validationName, fe.Field())
		return t
	})
}

func ValidateRequired(field validator.FieldLevel) bool {
	parsedIP := net.ParseIP(field.Field().String())
	return parsedIP != nil
}

func ValidateIP(field validator.FieldLevel) bool {
	parsedIP := net.ParseIP(field.Field().String())
	return parsedIP != nil
}

func ValidateURL(field validator.FieldLevel) bool {
	value := normalizeURL(field.Field().String())
	parsedURL, err := url.Parse(value)
	if err != nil {
		return false
	}

	// localhost URLs that do not start with http:// are interpreted
	// with `localhost` as the Scheme, not the Host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return false
	}

	return true
}

func ValidateLimitedRoute(field validator.FieldLevel) bool {
	for _, route := range atc.Routes {
		if route.Name == field.Field().String() {
			return true
		}
	}

	for _, supportedAction := range wrappa.SupportedActions {
		if field.Field().String() == supportedAction {
			return true
		}
	}

	return false
}

func ValidateEmptyTLSBindPort(field validator.FieldLevel) bool {
	if field.Field().Interface() == 0 {

		certField := field.Parent().FieldByName("Cert")
		keyField := field.Parent().FieldByName("Key")
		letsEncryptEnabledField := field.Top().FieldByName("LetsEncrypt").FieldByName("Enable")

		if !certField.IsValid() || !keyField.IsValid() || !letsEncryptEnabledField.IsValid() {
			return false
		}

		if certField.String() != "" || keyField.String() != "" || letsEncryptEnabledField.Bool() {
			return false
		}
	}

	return true
}

func ValidateEnabledLetsEncrypt(field validator.FieldLevel) bool {
	if field.Field().Bool() {
		tlsFields := field.Top().FieldByName("TLS")
		certField := tlsFields.FieldByName("Cert")
		keyField := tlsFields.FieldByName("Key")

		if !certField.IsValid() || !keyField.IsValid() {
			return false
		}

		if tlsFields.FieldByName("Cert").String() != "" || tlsFields.FieldByName("Key").String() != "" {
			return false
		}
	}

	return true
}

func ValidateTLSCertKey(field validator.FieldLevel) bool {
	keyField := field.Parent().FieldByName("key")
	if !keyField.IsValid() {
		return false
	}

	if field.Field().String() != "" && keyField.String() != "" {
		externalURLField := field.Top().FieldByName("ExternalURL")
		if !externalURLField.IsValid() {
			return false
		}

		value := strings.TrimRight(externalURLField.String(), "/")
		parsedURL, err := url.Parse(value)
		if err != nil {
			return false
		}

		if parsedURL.Scheme != "https" {
			return false
		}
	}

	return true
}

func ValidateTLS(field validator.FieldLevel) bool {
	certField := field.Parent().FieldByName("Cert")
	letsEncryptEnabledField := field.Top().FieldByName("LetsEncrypt").FieldByName("Enable")

	if !certField.IsValid() || !letsEncryptEnabledField.IsValid() {
		return false
	}

	if field.Field().String() == "" && certField.String() == "" || !letsEncryptEnabledField.Bool() {
		return false
	}

	return true
}

func ValidateRBAC(field validator.FieldLevel) bool {
	path := field.Field().String()
	if path == "" {
		return true
	}

	content, err := ioutil.ReadFile(path)
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

func normalizeURL(urlIn string) string {
	return strings.TrimRight(urlIn, "/")
}
