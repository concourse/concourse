set PATH=C:\Go\bin;%PATH%

set GOPATH=%CD%\gopath;%CD%\concourse;%CD%\gopath\src\github.com\vito\houdini\deps
set PATH=%CD%\gopath\bin;%PATH%

set /p FinalVersion=<final-version\version
set /p WorkerVersion=<concourse\src\worker-version\version

mkdir cli-artifacts
move fly-rc\fly_* cli-artifacts

go get github.com/jteeuwen/go-bindata

go build -o go-bindata.exe github.com/jteeuwen/go-bindata/go-bindata

.\go-bindata.exe -pkg bindata -o concourse\src\github.com\concourse\bin\bindata\bindata.go cli-artifacts/...

go build -ldflags "-X main.Version=%FinalVersion% -X github.com/concourse/atc/atccmd.Version=%FinalVersion% -X github.com/concourse/atc/atccmd.WorkerVersion=%WorkerVersion% -X main.WorkerVersion=%WorkerVersion%" -o .\binary\concourse_windows_amd64.exe github.com/concourse/bin/cmd/concourse
