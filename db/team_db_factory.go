package db

//go:generate counterfeiter . TeamDBFactory

type TeamDBFactory interface {
	GetTeamDB(string) TeamDB
}

type teamDBFactory struct {
	conn           Conn
	buildDBFactory BuildDBFactory
}

func NewTeamDBFactory(conn Conn, buildDBFactory BuildDBFactory) TeamDBFactory {
	return &teamDBFactory{
		conn:           conn,
		buildDBFactory: buildDBFactory,
	}
}

func (f *teamDBFactory) GetTeamDB(teamName string) TeamDB {
	return &teamDB{
		teamName:       teamName,
		conn:           f.conn,
		buildDBFactory: f.buildDBFactory,
	}
}
