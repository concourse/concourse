$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

# don't use tar.exe - it fails on funny-looking files
$env:Path = ($env:Path.Split(';') | Where-Object { $_ -ne 'c:\var\vcap\bosh\bin' }) -join ';'

C:\var\vcap\packages\houdini-windows\bin\houdini.exe `
  -containerGraceTime 0 `
  -depot C:\var\vcap\data\houdini-windows\containers `
  -listenAddr <%= p("bind_ip") %>:<%= p("bind_port") %>
