package helpers

import (
	"os"
	"strconv"
)

var storedPubliclyViewable *bool

func PipelinesPubliclyViewable() (bool, error) {
	if storedPubliclyViewable != nil {
		return *storedPubliclyViewable, nil
	}

	publiclyViewable, err := strconv.ParseBool(os.Getenv("PIPELINES_PUBLICLY_VIEWABLE"))
	if err != nil {
		return false, err
	}

	storedPubliclyViewable = &publiclyViewable
	return publiclyViewable, nil
}
