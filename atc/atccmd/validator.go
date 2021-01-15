package atccmd

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"github.com/spf13/viper"
)

func NewValidator() *validator.Validate {
	validate := validator.New()

	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")
	en_translations.RegisterDefaultTranslations(validate, trans)

	// XXX: Can we have better error messages for these?
	validate.RegisterValidation("ip", ValidateIP)
	validate.RegisterValidation("file", ValidateFile)
	validate.RegisterValidation("url", ValidateURL)
	validate.RegisterValidation("dir", ValidateDir)
	validate.RegisterValidation("limited_route", ValidateLimitedRoute)
	validate.RegisterValidation("private_key", ValidatePrivateKey)

	// XXX: TEST THIS
	validate.RegisterTranslation("limited_route", trans, func(ut ut.Translator) error {
		return ut.Add("limited_route", fmt.Sprintf("Not a valid route to limit. Valid routes include %v.", supportedActions), true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("limited_route", fe.Field())
		return t
	})

	return validate
}

func ValidateRequired(field validator.FieldLevel) bool {
	parsedIP := net.ParseIP(field.Field().String())
	return parsedIP != nil
}

func ValidateIP(field validator.FieldLevel) bool {
	parsedIP := net.ParseIP(field.Field().String())
	return parsedIP != nil
}

func ValidateFile(field validator.FieldLevel) bool {
	value := viper.GetString(field.Field().String())
	if value == "" {
		return false
	}

	stat, err := os.Stat(value)
	if err != nil {
		return false
	}

	if stat.IsDir() {
		return false
	}

	return true
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

func ValidateDir(field validator.FieldLevel) bool {
	stat, err := os.Stat(field.Field().String())
	if err == nil {
		if !stat.IsDir() {
			return false
		}
	}

	return true
}

var supportedActions = []string{atc.ListAllJobs}

func ValidateLimitedRoute(field validator.FieldLevel) bool {
	for _, route := range atc.Routes {
		if route.Name == field.Field().String() {
			return true
		}
	}

	for _, supportedAction := range supportedActions {
		if field.Field().String() == supportedAction {
			return true
		}
	}

	return false
}

func ValidatePrivateKey(field validator.FieldLevel) bool {
	rsaKeyBlob, err := ioutil.ReadFile(field.Field().String())
	if err != nil {
		return false
	}

	_, err = jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
	if err != nil {
		return false
	}

	return true
}

// XXX: Just convert into Cipher without validate
// func ValidateCiper(field validator.FieldLevel) bool {
// 	block, err := aes.NewCipher([]byte(field.Field().String()))
// 	if err != nil {
// 		return fmt.Errorf("failed to construct AES cipher: %s", err)
// 	}

// 	AEAD, err := cipher.NewGCM(block)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to construct GCM: %s", err)
// 	}

// 	return AEAD, nil
// }

func (v *Validator) GetPort(field string) (uint16, error) {
	value := viper.GetInt(field)

	port := uint16(value)

	// If the port value is not 4 digits, fail
	if int(port) != value {
		return 0, fmt.Errorf("%s - port '%s' is invalid", field, value)
	}

	return port, nil
}

func (c *NewRunCommand) GetDir(field string) (string, error) {
	value := viper.GetString(field)
	if value == "" {
		return "", nil
	}

	stat, err := os.Stat(value)
	if err == nil {
		if !stat.IsDir() {
			return "", fmt.Errorf("path '%s' is not a directory", value)
		}
	}

	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}

	return abs, nil
}

func (c *NewRunCommand) GetURL(field string) (*url.URL, error) {
	value := viper.GetString(field)
	if value == "" {
		return nil, nil
	}

	value = normalizeURL(value)
	parsedURL, err := url.Parse(value)

	if err != nil {
		return nil, err
	}

	// localhost URLs that do not start with http:// are interpreted
	// with `localhost` as the Scheme, not the Host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("%s - missing scheme in '%s'", field, value)
	}

	return parsedURL, nil
}

// XXX: is there a better way to do this?
func (c *NewRunCommand) GetSSLMode(field string) (string, error) {
	value := viper.GetString(field)
	switch value {
	case "disable", "require", "verify-ca", "verify-full":
		return value, nil
	default:
		return "", fmt.Errorf("%s - invalid choice %s", field, value)
	}
}

func (c *NewRunCommand) GetContainerPlacementStrategy(field string) (string, error) {
	value := viper.GetString(field)
	switch value {
	case "volume-locality", "random", "fewest-build-containers", "limit-active-tasks":
		return value, nil
	default:
		return "", fmt.Errorf("%s - invalid choice %s", field, value)
	}
}

func normalizeURL(urlIn string) string {
	return strings.TrimRight(urlIn, "/")
}

