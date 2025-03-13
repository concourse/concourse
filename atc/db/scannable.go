package db

type scannable interface {
	Scan(destinations ...any) error
}
