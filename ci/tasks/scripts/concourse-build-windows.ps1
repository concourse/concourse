trap {
  write-error $_
  exit 1
}

$env:Path += ";C:\Go\bin;C:\Program Files\Git\cmd;C:\tools\mingw64\bin"

$env:GOPATH = "$pwd\gopath"
$env:Path += ";$pwd\gopath\bin"

$version = "0.0.0"
if ([System.IO.File]::Exists("version\version")) {
  $version = (Get-Content "version\version")
}

# can't figure out how to pass an empty string arg in PowerShell, so just
# configure a noop for the fallback
$ldflags = "-X noop.Noop=noop"
if ([System.IO.File]::Exists("final-version\version")) {
  [string]$FinalVersion = (Get-Content "final-version\version")
  $ldflags = "-X github.com/concourse/concourse.Version=$FinalVersion"
}

Push-Location concourse
  go build -ldflags "$ldflags" -o concourse.exe ./bin/cmd/concourse
  mv concourse.exe ..\concourse-windows
Pop-Location

Push-Location concourse-windows
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

  Compress-Archive `
    -LiteralPath .\concourse.exe, .\fly-assets `
    -DestinationPath ".\concourse-${version}-windows-amd64.zip"

  Get-FileHash -Algorithm SHA1 -LiteralPath .\concourse-windows-amd64.zip | `
    Out-File -Encoding utf8 .\concourse-windows-amd64.zip.sha1

  Remove-Item .\concourse.exe, .\fly-assets -Recurse
Pop-Location
