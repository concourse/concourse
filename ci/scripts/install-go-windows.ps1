Set-PSDebug -trace 2 -strict

cd .\golang-windows

Remove-Item -Recurse -Force $env:USERPROFILE\go

Add-Type -AssemblyName System.IO.Compression.FileSystem

$zipfile = Get-Item "go*.zip"
[System.IO.Compression.ZipFile]::ExtractToDirectory($zipfile, $env:USERPROFILE)
