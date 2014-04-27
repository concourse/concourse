# Each Warden container is a /30 in Warden's network range, which is
# configured as 10.244.0.0/22. There are 256 available entries.
#
# We want two subnets, so I've arbitrarily divided this in half for each.
#
# consul will be 10.244.8.0/23
#
# Each network will have 128 subnets, and the first half of each subnet will
# be given static IPs.

require "yaml"
require "netaddr"

consul_subnets = []
consul_start = NetAddr::CIDR.create("10.244.8.0/30")

128.times do
  consul_subnets << consul_start
  consul_start = NetAddr::CIDR.create(consul_start.next_subnet)
end

puts YAML.dump(
  "networks" => [
    { "name" => "consul",
      "subnets" => consul_subnets.collect.with_index do |subnet, idx|
        { "cloud_properties" => {
            "name" => "random",
          },
          "range" => subnet.to_s,
          "reserved" => [subnet[1].ip],
          "static" => idx < 64 ? [subnet[2].ip] : [],
        }
      end
    },
  ])
