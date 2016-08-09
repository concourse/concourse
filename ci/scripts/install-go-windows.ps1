trap {
  write-error $_
  exit 1
}

Set-PSDebug -trace 2 -strict

echo what
echo the
echo deuce

Remove-Item -Recurse -Force $env:USERPROFILE\go

Add-Type -AssemblyName System.IO.Compression.FileSystem

$zipfile = Get-Item ".\golang-windows\go*.zip"
[System.IO.Compression.ZipFile]::ExtractToDirectory($zipfile, $env:USERPROFILE)
