$ErrorActionPreference = "Stop"
trap { $host.SetShouldExit(1) }

$env:Path += ";C:\Go\bin;C:\Program Files\Git\cmd"

$env:GOPATH = "$pwd\gopath"
$env:Path += ";$pwd\gopath\bin"

# can't figure out how to pass an empty string arg in PowerShell, so just
# configure a noop for the fallback
$ldflags = "-X noop.Noop=noop"
if ([System.IO.File]::Exists("final-version\version")) {
  [string]$FinalVersion = (Get-Content "final-version\version")
  $ldflags = "-X github.com/concourse/concourse.Version=$FinalVersion"
}

Push-Location concourse
  go build -ldflags "$ldflags" -o fly.exe ./fly
  mv fly.exe ..\fly-windows
Pop-Location

Push-Location fly-windows
  Compress-Archive `
    -LiteralPath .\fly.exe `
    -DestinationPath .\fly-windows-amd64.zip

  Get-FileHash -Algorithm SHA1 -LiteralPath .\fly-windows-amd64.zip | `
    Out-File -Encoding utf8 .\fly-windows-amd64.zip.sha1
Pop-Location
