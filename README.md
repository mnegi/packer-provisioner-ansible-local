# Ansible Local Provisioner

Type: ansible-local

The local Ansible provisioner configures Ansible to run on the machine by
Packer from local playbook and role files. Playbooks and roles can be
uploaded from your local machine to the remote machine. Ansible is run in
local mode via the ansible-playbook command.

## Basic Example

## Configuration Reference

## Execute Command

By default, Packer uses the following command to execute Ansible:

    ansible-playbook {{.PlaybookFile}} --verbose --connection=local
