$ErrorActionPreference = "Stop"
trap { $host.SetShouldExit(1) }

Add-Type -AssemblyName System.IO.Compression.FileSystem

echo "installing chocolatey"

$securityProtocolSettingsOriginal = [System.Net.ServicePointManager]::SecurityProtocol

# Set TLS 1.2 (3072), then TLS 1.1 (768), then TLS 1.0 (192), finally SSL 3.0 (48)
# Use integers because the enumeration values for TLS 1.2 and TLS 1.1 won't
# exist in .NET 4.0, even though they are addressable if .NET 4.5+ is
# installed (.NET 4.5 is an in-place upgrade).
[System.Net.ServicePointManager]::SecurityProtocol = 3072 -bor 768 -bor 192 -bor 48

Set-ExecutionPolicy Bypass -Scope LocalMachine -Force
Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))

[System.Net.ServicePointManager]::SecurityProtocol = $securityProtocolSettingsOriginal

choco install --force -y $Env:PACKAGE
