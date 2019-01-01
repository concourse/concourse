package healthcheck

import (
	uuid "github.com/nu7hatch/gouuid"
	"github.com/pkg/errors"
)

func createHandle() (string, error) {
	u4, err := uuid.NewV4()
	if err != nil {
		return "", errors.Wrapf(err,
			"couldn't create new uuid")
	}

	return "health-check-" + u4.String(), nil
}
