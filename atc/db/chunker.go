package db

const (
	maxDatabaseParameterLimit = 65000 // Postgres will error if more than 65535 are used.
)

func chunkByMaxParams(a []int, f func([]int) error) error {
	for {
		if len(a) <= maxDatabaseParameterLimit {
			return f(a)
		}
		err := f(a[:maxDatabaseParameterLimit])
		if err != nil {
			return err
		}
		a = a[maxDatabaseParameterLimit:]
	}
}
