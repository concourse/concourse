package configserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/mitchellh/mapstructure"
	"github.com/pivotal-golang/lager"
	"gopkg.in/yaml.v2"
)

func (s *Server) SaveConfig(w http.ResponseWriter, r *http.Request) {
	session := s.logger.Session("set-config")

	configIDStr := r.Header.Get(atc.ConfigIDHeader)
	if len(configIDStr) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "no config ID specified")
		return
	}

	var id db.ConfigID
	_, err := fmt.Sscanf(configIDStr, "%d", &id)
	if err != nil {
		session.Error("malformed-config-id", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "config ID is malformed: %s", err)
		return
	}

	var configStructure interface{}

	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/json":
		err = json.NewDecoder(r.Body).Decode(&configStructure)
	case "application/x-yaml":
		var body []byte
		body, err = ioutil.ReadAll(r.Body)
		if err == nil {
			err = yaml.Unmarshal(body, &configStructure)
		}
	default:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	if err != nil {
		session.Error("malformed-request-payload", err, lager.Data{
			"content-type": contentType,
		})

		w.WriteHeader(http.StatusBadRequest)
		return
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
		session.Error("failed-to-construct-decoder", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := decoder.Decode(configStructure); err != nil {
		session.Error("could-not-decode", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(md.Unused) != 0 {
		session.Error("extra-keys", err, lager.Data{
			"unused-keys": md.Unused,
		})
		w.WriteHeader(http.StatusBadRequest)

		fmt.Fprintln(w, "unknown/extra keys:")
		for _, unusedKey := range md.Unused {
			fmt.Fprintf(w, "  - %s\n", unusedKey)
		}
		return
	}

	err = s.validate(config)
	if err != nil {
		session.Error("ignoring-invalid-config", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s", err)
		return
	}

	session.Info("saving")

	err = s.db.SaveConfig(config, id)
	if err != nil {
		session.Error("failed-to-save-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to save config: %s", err)
		return
	}

	session.Info("saved")

	w.WriteHeader(http.StatusOK)
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
