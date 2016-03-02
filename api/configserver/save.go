package configserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/mitchellh/mapstructure"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"
)

var (
	ErrStatusUnsupportedMediaType = errors.New("content-type is not supported")
	ErrCannotParseContentType     = errors.New("content-type header could not be parsed")
	ErrMalformedRequestPayload    = errors.New("data in body could not be decoded")
	ErrFailedToConstructDecoder   = errors.New("decoder could not be constructed")
	ErrCouldNotDecode             = errors.New("data could not be decoded into config structure")
	ErrInvalidPausedValue         = errors.New("invalid paused value")
)

type ExtraKeysError struct {
	extraKeys []string
}

func (eke ExtraKeysError) Error() string {
	msg := &bytes.Buffer{}

	fmt.Fprintln(msg, "unknown/extra keys:")
	for _, unusedKey := range eke.extraKeys {
		fmt.Fprintf(msg, "  - %s\n", unusedKey)
	}

	return msg.String()
}

type SaveConfigResponse struct {
	Errors []string `json:"errors,omitempty"`
}

func newSaveConfigResponse(errorMessages []string) SaveConfigResponse {
	return SaveConfigResponse{
		Errors: errorMessages,
	}
}

func (s *Server) SaveConfig(w http.ResponseWriter, r *http.Request) {
	session := s.logger.Session("set-config")

	configVersionStr := r.Header.Get(atc.ConfigVersionHeader)
	if len(configVersionStr) == 0 {
		s.handleBadRequest(w, []string{"no config version specified"}, session)
		return
	}

	var version db.ConfigVersion
	_, err := fmt.Sscanf(configVersionStr, "%d", &version)
	if err != nil {
		session.Error("malformed-config-version", err)
		s.handleBadRequest(w, []string{fmt.Sprintf("config version is malformed: %s", err)}, session)
		return
	}

	config, pausedState, err := saveConfigRequestUnmarshaler(r)

	switch err {
	case ErrStatusUnsupportedMediaType:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	case ErrMalformedRequestPayload:
		session.Error("malformed-request-payload", err, lager.Data{
			"content-type": r.Header.Get("Content-Type"),
		})

		s.handleBadRequest(w, []string{"malformed config"}, session)
		return
	case ErrFailedToConstructDecoder:
		session.Error("failed-to-construct-decoder", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	case ErrCouldNotDecode:
		session.Error("could-not-decode", err)
		s.handleBadRequest(w, []string{"failed to decode config"}, session)
		return
	case ErrInvalidPausedValue:
		session.Error("invalid-paused-value", err)
		s.handleBadRequest(w, []string{"invalid paused value"}, session)
		return
	default:
		if err != nil {
			if eke, ok := err.(ExtraKeysError); ok {
				s.handleBadRequest(w, []string{eke.Error()}, session)
			} else {
				session.Error("unexpected-error", err)
				w.WriteHeader(http.StatusInternalServerError)
			}

			return
		}
	}

	errorMessages := s.validate(config)
	if len(errorMessages) > 0 {
		session.Error("ignoring-invalid-config", err)
		s.handleBadRequest(w, errorMessages, session)
		return
	}

	session.Info("saving")

	pipelineName := rata.Param(r, "pipeline_name")
	_, created, err := s.db.SaveConfig(atc.DefaultTeamName, pipelineName, config, version, pausedState)
	if err != nil {
		session.Error("failed-to-save-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to save config: %s", err)
		return
	}

	session.Info("saved")

	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleBadRequest(w http.ResponseWriter, errorMessages []string, session lager.Logger) {
	saveConfigResponse := newSaveConfigResponse(errorMessages)
	responseJSON, err := json.Marshal(saveConfigResponse)
	if err != nil {
		session.Error("failed-to-marshal-validation-response", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to generate error response: %s", err)
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	w.Write(responseJSON)
}

func requestToConfig(contentType string, requestBody io.ReadCloser, configStructure interface{}) (db.PipelinePausedState, error) {
	pausedState := db.PipelineNoChange

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return db.PipelineNoChange, ErrCannotParseContentType
	}

	switch mediaType {
	case "application/json":
		err := json.NewDecoder(requestBody).Decode(configStructure)
		if err != nil {
			return db.PipelineNoChange, ErrMalformedRequestPayload
		}

	case "application/x-yaml":
		body, err := ioutil.ReadAll(requestBody)
		if err == nil {
			err = yaml.Unmarshal(body, configStructure)
		}

		if err != nil {
			return db.PipelineNoChange, ErrMalformedRequestPayload
		}

	case "multipart/form-data":
		multipartReader := multipart.NewReader(requestBody, params["boundary"])

		for {
			part, err := multipartReader.NextPart()

			if err == io.EOF {
				break
			}

			if err != nil {
				return db.PipelineNoChange, err
			}

			if part.FormName() == "paused" {
				pausedValue, err := ioutil.ReadAll(part)
				if err != nil {
					return db.PipelineNoChange, err
				}

				if string(pausedValue) == "true" {
					pausedState = db.PipelinePaused
				} else if string(pausedValue) == "false" {
					pausedState = db.PipelineUnpaused
				} else {
					return db.PipelineNoChange, ErrInvalidPausedValue
				}
			} else {
				partContentType := part.Header.Get("Content-type")
				_, err := requestToConfig(partContentType, part, configStructure)
				if err != nil {
					return db.PipelineNoChange, ErrMalformedRequestPayload
				}
			}
		}
	default:
		return db.PipelineNoChange, ErrStatusUnsupportedMediaType
	}

	return pausedState, nil
}

func saveConfigRequestUnmarshaler(r *http.Request) (atc.Config, db.PipelinePausedState, error) {
	var configStructure interface{}
	pausedState, err := requestToConfig(r.Header.Get("Content-Type"), r.Body, &configStructure)
	if err != nil {
		return atc.Config{}, db.PipelineNoChange, err
	}

	var config atc.Config
	var md mapstructure.Metadata
	msConfig := &mapstructure.DecoderConfig{
		Metadata:         &md,
		Result:           &config,
		WeaklyTypedInput: true,
		DecodeHook:       atc.SanitizeDecodeHook,
	}
	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		return atc.Config{}, db.PipelineNoChange, ErrFailedToConstructDecoder
	}

	if err := decoder.Decode(configStructure); err != nil {
		return atc.Config{}, db.PipelineNoChange, ErrCouldNotDecode
	}

	if len(md.Unused) != 0 {
		return atc.Config{}, db.PipelineNoChange, ExtraKeysError{extraKeys: md.Unused}
	}

	return config, pausedState, nil
}
