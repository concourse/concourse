package api

import (
	"errors"
	"net/url"
	"strings"

	"github.com/concourse/baggageclaim/volume"
)

func ConvertQueryToProperties(values url.Values) (volume.Properties, error) {
	properties := volume.Properties{}

	for name, value := range values {
		if len(value) > 1 {
			err := errors.New("a property may only have a single value: " + name + " has many (" + strings.Join(value, ", ") + ")")
			return volume.Properties{}, err
		}

		properties[name] = value[0]
	}

	return properties, nil
}
