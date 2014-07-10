Vagrant.configure("2") do |config|
  config.vm.box = "hashicorp/precise64"

  config.vm.network "forwarded_port", guest: 8080, host: 8080 # atc
  config.vm.network "forwarded_port", guest: 5637, host: 5637 # glider
  config.vm.network "forwarded_port", guest: 7777, host: 7777 # warden (debugging)

  config.vm.provider "virtualbox" do |v|
    v.memory = 4096
    v.cpus = 2
  end
  
  config.vm.provider "vmware_fusion" do |v, override|
    v.vmx["memsize"] = 4096    
    v.vmx["numvcpus"] = 2
  end

  config.vm.provision "bosh" do |c|
    c.manifest = File.read("manifests/vagrant-bosh.yml")
  end
end
