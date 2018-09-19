set PATH=C:\Go\bin;C:\Program Files\Git\cmd;%PATH%

set GOPATH=%CD%\gopath
set PATH=%CD%\gopath\bin;%PATH%

cd .\concourse\fly

go mod download

go install github.com/onsi/ginkgo/ginkgo

ginkgo -r -p
