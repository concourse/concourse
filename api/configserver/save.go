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
	"reflect"
	"time"

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

func (s *Server) SaveConfig(w http.ResponseWriter, r *http.Request) {
	session := s.logger.Session("set-config")

	configVersionStr := r.Header.Get(atc.ConfigVersionHeader)
	if len(configVersionStr) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "no config version specified")
		return
	}

	var version db.ConfigVersion
	_, err := fmt.Sscanf(configVersionStr, "%d", &version)
	if err != nil {
		session.Error("malformed-config-version", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "config version is malformed: %s", err)
		return
	}

	config, pausedState, err := saveConfigRequestUnmarshler(r)

	switch err {
	case ErrStatusUnsupportedMediaType:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	case ErrMalformedRequestPayload:
		session.Error("malformed-request-payload", err, lager.Data{
			"content-type": r.Header.Get("Content-Type"),
		})

		w.WriteHeader(http.StatusBadRequest)
		return
	case ErrFailedToConstructDecoder:
		session.Error("failed-to-construct-decoder", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	case ErrCouldNotDecode:
		session.Error("could-not-decode", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	case ErrInvalidPausedValue:
		session.Error("invalid-paused-value", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid paused value")
		return
	default:
		if err != nil {
			if eke, ok := err.(ExtraKeysError); ok {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintln(w, eke)
			} else {
				session.Error("unexpected-error", err)
				w.WriteHeader(http.StatusInternalServerError)
			}

			return
		}
	}

	err = s.validate(config)
	if err != nil {
		session.Error("ignoring-invalid-config", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s", err)
		return
	}

	session.Info("saving")

	pipelineName := rata.Param(r, "pipeline_name")
	created, err := s.db.SaveConfig(pipelineName, config, version, pausedState)
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
		w.WriteHeader(http.StatusOK)
	}
}

func requestToConfig(contentType string, requestBody io.ReadCloser) (interface{}, db.PipelinePausedState, error) {
	var err error
	var configStructure interface{}
	pausedState := db.PipelineNoChange

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return atc.Config{}, db.PipelineNoChange, ErrCannotParseContentType
	}

	switch mediaType {
	case "application/json":
		err = json.NewDecoder(requestBody).Decode(&configStructure)
	case "application/x-yaml":
		var body []byte
		body, err = ioutil.ReadAll(requestBody)
		if err == nil {
			err = yaml.Unmarshal(body, &configStructure)
		}
	case "multipart/form-data":
		err = json.NewDecoder(requestBody).Decode(&configStructure)
		multipartReader := multipart.NewReader(requestBody, params["boundary"])

		for {
			part, err := multipartReader.NextPart()

			if err == io.EOF {
				break
			}

			if err != nil {
				return atc.Config{}, db.PipelineNoChange, err
			}

			if part.FormName() == "paused" {
				pausedValue, err := ioutil.ReadAll(part)
				if err != nil {
					return atc.Config{}, db.PipelineNoChange, err
				}

				if string(pausedValue) == "true" {
					pausedState = db.PipelinePaused
				} else if string(pausedValue) == "false" {
					pausedState = db.PipelineUnpaused
				} else {
					return atc.Config{}, db.PipelineNoChange, ErrInvalidPausedValue
				}
			} else {
				partContentType := part.Header.Get("Content-type")
				configStructure, _, err = requestToConfig(partContentType, part)
			}
		}
	default:
		return atc.Config{}, db.PipelineNoChange, ErrStatusUnsupportedMediaType
	}

	return configStructure, pausedState, nil
}

func saveConfigRequestUnmarshler(r *http.Request) (atc.Config, db.PipelinePausedState, error) {
	configStructure, pausedState, err := requestToConfig(r.Header.Get("Content-Type"), r.Body)
	if err != nil {
		return atc.Config{}, db.PipelineNoChange, err
	}

	var config atc.Config
	var md mapstructure.Metadata
	msConfig := &mapstructure.DecoderConfig{
		Metadata:         &md,
		Result:           &config,
		WeaklyTypedInput: true,
		DecodeHook: func(
			dataKind reflect.Kind,
			valKind reflect.Kind,
			data interface{},
		) (interface{}, error) {
			if valKind == reflect.Map {
				if dataKind == reflect.Map {
					return sanitize(data)
				}
			}

			if dataKind == reflect.String {
				val, err := time.ParseDuration(data.(string))
				if err == nil {
					return val, nil
				}
			}

			if valKind == reflect.String {
				if dataKind == reflect.String {
					return data, nil
				}

				// format it as JSON/YAML would
				return json.Marshal(data)
			}

			return data, nil
		},
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

func sanitize(root interface{}) (interface{}, error) {
	switch rootVal := root.(type) {
	case map[interface{}]interface{}:
		sanitized := map[string]interface{}{}

		for key, val := range rootVal {
			str, ok := key.(string)
			if !ok {
				return nil, errors.New("non-string key")
			}

			sub, err := sanitize(val)
			if err != nil {
				return nil, err
			}

			sanitized[str] = sub
		}

		return sanitized, nil

	default:
		return rootVal, nil
	}

	return nil, errors.New(fmt.Sprintf("unknown type (%s) during sanitization: %#v\n", reflect.TypeOf(root).String(), root))
}
