package gcpsecretmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"regexp"
	"time"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	lager "code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds"
	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . SecretManagerAPI
type SecretManagerAPI interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

// secretIDPattern is the character set Google Secret Manager permits in a
// secret ID. Validating before building the slash-delimited resource name
// stops a crafted ((var)) from escaping its secret into another resource.
var secretIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,255}$`)

var ErrInvalidSecretID = errors.New("invalid Google Secret Manager secret ID")

type SecretID string

func NewSecretID(proposed string) (SecretID, error) {
	if !secretIDPattern.MatchString(proposed) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSecretID, proposed)
	}
	return SecretID(proposed), nil
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

type SecretManager struct {
	log             lager.Logger
	api             SecretManagerAPI
	projectID       string
	secretVersion   string
	requestTimeout  time.Duration
	secretTemplates []*creds.SecretTemplate
}

func NewSecretManager(
	log lager.Logger,
	api SecretManagerAPI,
	projectID string,
	secretVersion string,
	requestTimeout time.Duration,
	secretTemplates []*creds.SecretTemplate,
) *SecretManager {
	// A zero timeout yields an already-expired context, so treat it as unset.
	if requestTimeout <= 0 {
		requestTimeout = DefaultRequestTimeout
	}

	return &SecretManager{
		log:             log,
		api:             api,
		projectID:       projectID,
		secretVersion:   secretVersion,
		requestTimeout:  requestTimeout,
		secretTemplates: secretTemplates,
	}
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (s *SecretManager) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, tmpl := range s.secretTemplates {
		if lPath := creds.NewSecretLookupWithTemplate(tmpl, teamName, pipelineName); lPath != nil {
			lookupPaths = append(lookupPaths, lPath)
		}
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (s *SecretManager) Get(secretPath string) (any, *time.Time, bool, error) {
	value, expiration, found, err := s.getSecretByID(secretPath)
	if err != nil {
		// Log the secret ID only, never the payload.
		s.log.Error("failed-to-fetch-gcp-secret", err, lager.Data{
			"secret-id": secretPath,
			"project":   s.projectID,
		})
		return nil, nil, false, err
	}
	if found {
		return value, expiration, true, nil
	}
	return nil, nil, false, nil
}

// getSecretByID looks up a secret version by its ID. A payload that parses as a
// JSON object is returned as a map[string]any so ((secret.field)) works;
// anything else is returned verbatim as a string. Expiration is always nil.
func (s *SecretManager) getSecretByID(secretPath string) (any, *time.Time, bool, error) {
	secretID, err := NewSecretID(secretPath)
	if err != nil {
		return nil, nil, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.requestTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", s.projectID, secretID, s.secretVersion)

	resp, err := s.api.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		// NotFound and FailedPrecondition (all versions destroyed/disabled)
		// mean "not here": fall through to the next lookup path.
		switch status.Code(err) {
		case codes.NotFound, codes.FailedPrecondition:
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}

	payload := resp.GetPayload()
	if payload == nil {
		return nil, nil, false, fmt.Errorf("secret version %q returned no payload", secretID)
	}

	data := payload.GetData()

	// Verify the server-supplied CRC32C so a corrupted payload fails loudly.
	if crc := payload.DataCrc32C; crc != nil {
		if uint32(*crc) != crc32.Checksum(data, crc32cTable) {
			return nil, nil, false, fmt.Errorf("data corruption detected for secret %q: crc32c checksum mismatch", secretID)
		}
	}

	if values, err := decodeJSONValue(data); err == nil {
		return values, nil, true, nil
	}

	return string(data), nil, true, nil
}

// decodeJSONValue returns the payload as a map only when it is a JSON object.
// Scalars, arrays and literal "null" are rejected so a bare string stays a string.
func decodeJSONValue(data []byte) (map[string]any, error) {
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, errors.New("payload is not a JSON object")
	}
	return values, nil
}
