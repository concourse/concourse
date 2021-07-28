set PATH=C:\Go\bin;C:\Program Files\Git\cmd;C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin;C:\Program Files (x86)\Windows Resource Kits\Tools;%PATH%

set GOPATH=%CD%\gopath
set PATH=%CD%\gopath\bin;%PATH%

cd .\baggageclaim

go mod download

go install github.com/onsi/ginkgo/ginkgo

ginkgo -r -p
