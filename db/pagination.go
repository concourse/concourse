package db

type Page struct {
	Since int
	Until int
	Limit int
}

type Pagination struct {
	Previous *Page
	Next     *Page
}
