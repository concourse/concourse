$ErrorActionPreference = "Stop"
trap { $host.SetShouldExit(1) }

$env:Path += ";C:\Go\bin;C:\Program Files\Git\cmd;C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin"

$env:GOPATH = "$pwd\gopath"
$env:Path += ";$pwd\gopath\bin"

$version = "0.0.0"
if (Test-Path "version\version") {
  $version = (Get-Content "version\version")
}

# can't figure out how to pass an empty string arg in PowerShell, so just
# configure a noop for the fallback
$ldflags = "-X noop.Noop=noop"
if (Test-Path "final-version\version") {
  $finalVersion = (Get-Content "final-version\version")
  $ldflags = "-X github.com/concourse/concourse.Version=$finalVersion"
}

Push-Location concourse
  go install github.com/gobuffalo/packr/packr
  packr build -o concourse.exe -ldflags "$ldflags" ./cmd/concourse
  mv concourse.exe ..\concourse-windows
Pop-Location

Push-Location concourse-windows
  mkdir bin
  mv concourse.exe bin

  mkdir fly-assets
  if (Test-Path "..\fly-linux") {
    cp ..\fly-linux\fly-*.tgz fly-assets
  }

  if (Test-Path "..\fly-windows") {
    cp ..\fly-windows\fly-*.zip fly-assets
  }

  if (Test-Path "..\fly-darwin") {
    cp ..\fly-darwin\fly-*.tgz fly-assets
  }

  mkdir concourse
  mv bin concourse
  mv fly-assets concourse

  Compress-Archive `
    -LiteralPath .\concourse `
    -DestinationPath ".\concourse-${version}-windows-amd64.zip"

  Get-FileHash -Algorithm SHA1 ".\concourse-${version}-windows-amd64.zip" | `
    Out-File -Encoding utf8 ".\concourse-${version}-windows-amd64.zip.sha1"

  Remove-Item .\concourse -Recurse
Pop-Location
