# Contributing

## go-bindata

This project basically consists of an http server that serves static files
(HTML, CSS and Javascript) which are themselves built from elm source code.
All these files get bundled into go source code using the library `go-bindata`,
so whenever you make changes to this repo, before committing you should run
`make clean all` and check in the updated `bindata/bindata.go`.
