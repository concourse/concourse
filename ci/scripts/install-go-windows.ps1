trap {
  write-error $_
  exit 1
}

Set-PSDebug -trace 2 -strict

echo what
Set-PSDebug -trace 2 -strict

echo the
Set-PSDebug -trace 2 -strict

echo deuce
Set-PSDebug -trace 2 -strict

Remove-Item -Recurse -Force $env:USERPROFILE\go

Set-PSDebug -trace 2 -strict

Add-Type -AssemblyName System.IO.Compression.FileSystem

Set-PSDebug -trace 2 -strict

$zipfile = Get-Item ".\golang-windows\go*.zip"
Set-PSDebug -trace 2 -strict

[System.IO.Compression.ZipFile]::ExtractToDirectory($zipfile, $env:USERPROFILE)
