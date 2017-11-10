package db

//go:generate counterfeiter . BaseResourceTypeFactory

type BaseResourceTypeFactory interface {
	Find(name string) (*UsedBaseResourceType, bool, error)
}

type baseResourceTypeFactory struct {
	conn Conn
}

func NewBaseResourceTypeFactory(conn Conn) BaseResourceTypeFactory {
	return &baseResourceTypeFactory{
		conn: conn,
	}
}

func (f *baseResourceTypeFactory) Find(name string) (*UsedBaseResourceType, bool, error) {
	brt := BaseResourceType{
		Name: name,
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	ubrt, found, err := brt.Find(tx)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return ubrt, found, nil
}
