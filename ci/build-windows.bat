set GOPATH=%CD%\gopath;%CD%\concourse;%CD%\gopath\src\github.com\vito\houdini\Godeps_windows\_workspace
set PATH=%CD%\gopath\bin;%PATH%

go build -o .\windows-binary\concourse_windows_amd64.exe github.com/vito/concourse-bin/cmd/concourse
