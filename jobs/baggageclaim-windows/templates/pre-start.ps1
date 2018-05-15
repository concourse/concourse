New-NetFirewallRule `
  -LocalPort <%= p("bind_port") %> `
  -Protocol TCP `
  -Direction Inbound `
  -Name baggageclaim `
  -DisplayName baggageclaim
