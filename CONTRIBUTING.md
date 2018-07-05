# Contributing

## go-bindata

All the web assets for the auth web pages get bundled into go source code using
the library `go-bindata`, so whenever you make changes to those files, before
committing you should run `make clean all` and check in the up-to-date
`bindata/bindata.go`.
