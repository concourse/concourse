$ErrorActionPreference = "Stop"
trap { $host.SetShouldExit(1) }

Add-Type -AssemblyName System.IO.Compression.FileSystem

$msi = Get-Item ".\golang-windows\go*.msi"

echo "installing $msi"

$p = Start-Process -FilePath "msiexec" -ArgumentList "/passive /norestart /i $msi" -Wait -PassThru

if ($p.ExitCode -ne 0) {
  throw "failed"
}

echo "done"
