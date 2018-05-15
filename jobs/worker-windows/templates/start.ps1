$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

<%
  tsa_addrs = nil
  tsa_port = nil

  if_p("tsa.host", "tsa.port") do |host, port|
    tsa_addrs = ["#{host}:#{port}"]
  end

  if tsa_addrs.nil? && tsa_port.nil?
    tsa = link("tsa")
    tsa_port = tsa.p("bind_port")
    tsa_addrs = tsa.instances.collect{|instance| "#{instance.address}:#{tsa_port}"}
  end

  garden_addr = ""

  if_p("garden.address") do |addr|
    garden_addr = addr
  end

  if garden_addr.empty?
    if_link("garden") do |garden|
      instance = garden.instances.find { |i| i.id == spec.id }
      if instance
        garden_addr = "#{instance.address}:#{garden.p("bind_port")}"
      end
    end
  end

  if garden_addr.empty?
    # no property and no link; assume it's colocated
    garden_addr = "#{spec.address}:7777"
  end

  baggageclaim_url = ""

  if_p("baggageclaim.url") do |url|
    baggageclaim_url = url
  end

  if baggageclaim_url.empty?
    if_link("baggageclaim") do |baggageclaim|
      instance = baggageclaim.instances.find { |i| i.id == spec.id }
      if instance
        baggageclaim_url = "http://#{instance.address}:#{baggageclaim.p("bind_port")}"
      end
    end
  end

  name_prefix = spec.id.split("-")[0]
%>

$WORKER_VERSION = Get-Content C:\var\vcap\packages\worker_version-windows\version

C:\var\vcap\packages\worker-windows\bin\worker.exe `
  start `
  --name "<%= name_prefix %>:$(hostname)" `
  --version $WORKER_VERSION `
  --registration-mode <%= p("tsa.registration_mode") %> `
  --garden-addr <%= garden_addr %> `
  --baggageclaim-url <%= baggageclaim_url %> `
  --tsa-public-key "C:\var\vcap\jobs\worker-windows\config\tsa_host_key.pub" `
  --tsa-worker-private-key "C:\var\vcap\jobs\worker-windows\config\worker_key" `
  --log-level <%= p("log_level") %> `
  --platform <%= p("platform") %> `
  <% if_p("team") do |team| %> `
    --team '<%= team %>' `
  <% end %> `
  <% p("tags").each do |tag| %> `
    --tag '<%= tag %>' `
  <% end %> `
  <% tsa_addrs.each do |tsa_addr| %> `
    --tsa-host <%= tsa_addr %> `
  <% end %> `
  <% if p("garden.forward_address", nil) %> `
    --garden-forward-addr <%= p("garden.forward_address") %> `
  <% end %> `
  <% if p("baggageclaim.forward_address", nil) %> `
    --baggageclaim-forward-addr <%= p("baggageclaim.forward_address") %> `
  <% end %> `
  <% if_p("http_proxy_url") do |url| %> `
    --http-proxy "<%= url %>" `
  <% end %> `
  <% if_p("https_proxy_url") do |url| %> `
    --https-proxy "<%= url %>" `
  <% end %> `
  <% if_p("no_proxy") do |url| %> `
    --no-proxy "<%= url.join(',') %>" `
  <% end %>
