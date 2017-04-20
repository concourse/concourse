package db

type Team struct {
	Name  string
	Admin bool
}

type SavedTeam struct {
	ID int
	Team
}
