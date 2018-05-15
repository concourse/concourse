$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

# don't use tar.exe - it fails on funny-looking files
$env:Path = ($env:Path.Split(';') | Where-Object { $_ -ne 'c:\var\vcap\bosh\bin' }) -join ';'

C:\var\vcap\packages\baggageclaim-windows\bin\baggageclaim.exe `
  --volumes C:\var\vcap\data\baggageclaim\volumes `
  --driver <%= p("driver") %> `
  --bind-ip <%= p("bind_ip") %> `
  --bind-port <%= p("bind_port") %> `
  --log-level <%= p("log_level") %>
