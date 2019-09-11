package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Check(check db.Check) atc.Check {

	atcCheck := atc.Check{
		ID:     check.ID(),
		Status: string(check.Status()),
	}

	if !check.CreateTime().IsZero() {
		atcCheck.CreateTime = check.CreateTime().Unix()
	}

	if !check.StartTime().IsZero() {
		atcCheck.StartTime = check.StartTime().Unix()
	}

	if !check.EndTime().IsZero() {
		atcCheck.EndTime = check.EndTime().Unix()
	}

	if err := check.CheckError(); err != nil {
		atcCheck.CheckError = err.Error()
	}

	return atcCheck
}
