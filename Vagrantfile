Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/trusty64"

  config.vm.network "forwarded_port", guest: 8080, host: 8080 # atc
  config.vm.network "forwarded_port", guest: 5637, host: 5637 # glider
  config.vm.network "forwarded_port", guest: 7777, host: 7777 # warden (debugging)

  config.vm.provider "virtualbox" do |v|
    v.memory = 4096
    v.cpus = 2
  end

  # provides aufs
  config.vm.provision "shell",
    inline: "apt-get -y install linux-image-extra-$(uname -r)"

  config.vm.provision "bosh" do |c|
    c.manifest = File.read("manifests/vagrant-bosh.yml")
  end
end
