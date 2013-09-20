package main

import (
	"github.com/mitchellh/packer/packer/plugin"
	"github.com/kelseyhightower/packer-provisioner-ansible-local/provisioner/ansible-local"
)

func main() {
	plugin.ServeProvisioner(new(ansiblelocal.Provisioner))
}
