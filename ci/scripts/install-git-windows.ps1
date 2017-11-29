trap {
  write-error $_
  exit 1
}

Add-Type -AssemblyName System.IO.Compression.FileSystem

echo "installing chocolatey"

Set-ExecutionPolicy Bypass -Scope LocalMachine -Force; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))

choco install --force -y git
