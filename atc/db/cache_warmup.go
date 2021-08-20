package db

import sq "github.com/Masterminds/squirrel"

func CacheWarmUp(runner sq.Runner) error {
	err := warmUpBaseResourceTypesCache(runner)
	if err != nil {
		return err
	}

	return nil
}
