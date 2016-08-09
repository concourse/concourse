set PATH=C:\Go\bin;%PATH

set GOPATH=%CD%\concourse
set PATH=%CD%\concourse\bin;%PATH%

go install github.com/onsi/ginkgo/ginkgo

cd .\concourse\src\github.com\concourse\fly

ginkgo -r -p
