package db

type Page struct {
	From int // inclusive
	To   int // inclusive

	Limit   int
	UseDate bool
}

type Pagination struct {
	Newer *Page
	Older *Page
}

