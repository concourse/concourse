package db

type scannable interface {
	Scan(destinations ...interface{}) error
}
