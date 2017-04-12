package dbng

//go:generate counterfeiter . Job

type Job interface {
	ID() int
}

var jobQuery = psql.Select("j.id").
	From("jobs j")

type job struct {
	id int

	conn Conn
}

func (j *job) ID() int { return j.id }

func scanJob(j *job, row scannable) error {
	err := row.Scan(&j.id)
	if err != nil {
		return err
	}

	return nil
}
