set GOPATH=%CD%\gopath;%CD%\concourse
set PATH=%CD%\gopath\bin;%PATH%

set /p FinalVersion=<final-version\version

go get github.com/jteeuwen/go-bindata

go build -o go-bindata.exe github.com/jteeuwen/go-bindata/go-bindata

.\go-bindata.exe -pkg bindata -o gopath\src\github.com\concourse\bin\bindata\bindata.go cli-artifacts/...

go build -ldflags "-X main.Version=%FinalVersion% -X github.com/concourse/atc/atccmd.Version=%FinalVersion%" -o .\binary\concourse_windows_amd64.exe github.com/concourse/bin/cmd/concourse
