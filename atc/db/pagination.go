package db

type Page struct {
	Since int // exclusive
	Until int // exclusive

	From int // inclusive
	To   int // inclusive

	Limit   int
	UseDate bool
}

type Pagination struct {
	Previous *Page
	Next     *Page
}
