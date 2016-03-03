set GOPATH=%CD%\gopath;%CD%\concourse;%CD%\gopath\src\github.com\vito\houdini\Godeps_windows\_workspace
set PATH=%CD%\gopath\bin;%PATH%

set /p FinalVersion=<final-version\number

go get github.com/jteeuwen/go-bindata

go build -o go-bindata.exe github.com/jteeuwen/go-bindata/go-bindata

.\go-bindata.exe -pkg bindata -o gopath\src\github.com\concourse\bin\bindata\bindata.go cli-artifacts/...

go build -ldflags "-X github.com/concourse/atc/atccmd.Version=%FinalVersion%" -o .\windows-binary\concourse_windows_amd64.exe github.com/concourse/bin/cmd/concourse
