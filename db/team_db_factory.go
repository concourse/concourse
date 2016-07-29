package db

//go:generate counterfeiter . TeamDBFactory

type TeamDBFactory interface {
	GetTeamDB(string) TeamDB
}

type teamDBFactory struct {
	conn Conn
	bus  *notificationsBus
}

func NewTeamDBFactory(conn Conn, bus *notificationsBus) TeamDBFactory {
	return &teamDBFactory{
		conn: conn,
		bus:  bus,
	}
}

func (f *teamDBFactory) GetTeamDB(teamName string) TeamDB {
	return &teamDB{
		teamName:     teamName,
		conn:         f.conn,
		buildFactory: newBuildFactory(f.conn, f.bus),
	}
}
