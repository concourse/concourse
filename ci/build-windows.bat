set GOPATH=%CD%\gopath;%CD%\concourse
set PATH=%CD%\gopath\bin;%PATH%

go build -o .\windows-binary\concourse_windows_amd64.exe \
  github.com/vito/concourse-bin/cmd/concourse
