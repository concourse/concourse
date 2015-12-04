package db

type Team struct {
	Name string
}

type SavedTeam struct {
	ID int
	Team
}
