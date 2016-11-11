package dbng

import sq "github.com/Masterminds/squirrel"

type Build struct {
	ID int
}

func (build *Build) Delete(tx Tx) (bool, error) {
	rows, err := psql.Delete("builds").
		Where(sq.Eq{
			"id": build.ID,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return false, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return false, err
	}

	if affected == 0 {
		panic("TESTME")
		return false, nil
	}

	return true, nil
}
