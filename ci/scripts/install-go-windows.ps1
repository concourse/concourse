Set-PSDebug -trace 2 -strict

Remove-Item -Recurse -Force $env:USERPROFILE\go

Add-Type -AssemblyName System.IO.Compression.FileSystem

$zipfile = Get-Item ".\golang-windows\go*.zip"
[System.IO.Compression.ZipFile]::ExtractToDirectory($zipfile, $env:USERPROFILE)
