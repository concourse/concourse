package db

type checkableCounter struct {
	conn Conn
}

func NewCheckableCounter(conn Conn) *checkableCounter {
	return &checkableCounter{
		conn: conn,
	}
}

// Returns the number of resource config scopes in the database. This
// represents the number of things that can be checked by lidar.
func (c *checkableCounter) CheckableCount() (int, error) {
	var checkableCount int

	err := psql.Select("COUNT(id)").
		From("resource_config_scopes").
		RunWith(c.conn).
		QueryRow().
		Scan(&checkableCount)
	return checkableCount, err
}
