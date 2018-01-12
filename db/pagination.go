package db

type Page struct {
	Since int // exclusive
	Until int // exclusive

	From int // inclusive
	To   int // inclusive

	Around int

	Limit int
}

type Pagination struct {
	Previous *Page
	Next     *Page
}
