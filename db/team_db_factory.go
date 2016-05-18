package db

//go:generate counterfeiter . TeamDBFactory

type TeamDBFactory interface {
	GetTeamDB(string) TeamDB
}

type teamDBFactory struct {
	conn Conn
}

func NewTeamDBFactory(conn Conn) TeamDBFactory {
	return &teamDBFactory{
		conn: conn,
	}
}

func (f *teamDBFactory) GetTeamDB(teamName string) TeamDB {
	return &teamDB{
		teamName: teamName,
		conn:     f.conn,
	}
}
