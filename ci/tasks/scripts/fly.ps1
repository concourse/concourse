Set-PSDebug -Trace 1 -Strict

trap {
  write-error $_
  exit 1
}

$env:Path += ";C:\Go\bin;C:\Program Files\Git\cmd"

$env:GOPATH = "$pwd\gopath"
$env:Path += ";$pwd\gopath\bin"

cd .\concourse\fly

go mod download

go install github.com/onsi/ginkgo/ginkgo

ginkgo -r -p
