$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

C:\var\vcap\packages\baggageclaim-windows\bin\baggageclaim.exe `
  --volumes C:\var\vcap\data\baggageclaim\volumes `
  --driver <%= p("driver") %> `
  --bind-ip <%= p("bind_ip") %> `
  --bind-port <%= p("bind_port") %> `
  --log-level <%= p("log_level") %>
