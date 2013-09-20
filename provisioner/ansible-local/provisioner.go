package ansiblelocal

import (
	"fmt"
	"github.com/mitchellh/packer/common"
	"github.com/mitchellh/packer/packer"
	"os"
	"path/filepath"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	tpl                 *packer.ConfigTemplate

	// The main playbook file to execute.
	PlaybookFile string `mapstructure:"playbook_file"`

	// An array of local paths of playbook files to upload.
	PlaybookPaths []string `mapstructure:"playbook_paths"`

	// An array of local paths of roles to upload.
	RolePaths []string `mapstructure:"role_paths"`

	// The directory where files will be uploaded. Packer requires write
	// permissions in this directory.
	StagingDir string `mapstructure:"staging_directory"`
}

type Provisioner struct {
	config Config
}

func (p *Provisioner) Prepare(raws ...interface{}) error {
	md, err := common.DecodeConfig(&p.config, raws...)
	if err != nil {
		return err
	}

	p.config.tpl, err = packer.NewConfigTemplate()
	if err != nil {
		return err
	}

	p.config.tpl.UserVars = p.config.PackerUserVars

	// Accumulate any errors
	errs := common.CheckUnusedConfig(md)

	if p.config.StagingDir == "" {
		p.config.StagingDir = "/tmp/packer-provisioner-ansible-local"
	}

	// Templates
	templates := map[string]*string{
		"staging_dir": &p.config.StagingDir,
	}

	for n, ptr := range templates {
		var err error
		*ptr, err = p.config.tpl.Process(*ptr, nil)
		if err != nil {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("Error processing %s: %s", n, err))
		}
	}

	// Validation
	err = validateFileConfig(p.config.PlaybookFile, "playbook_file", true)
	if err != nil {
		errs = packer.MultiErrorAppend(errs, err)
	}

	for _, path := range p.config.PlaybookPaths {
		err := validateDirConfig(path, "playbook_paths")
		if err != nil {
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	for _, path := range p.config.RolePaths {
		if err := validateDirConfig(path, "role_paths"); err != nil {
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	if errs != nil && len(errs.Errors) > 0 {
		return errs
	}

	return nil
}

func (p *Provisioner) Provision(ui packer.Ui, comm packer.Communicator) error {
	ui.Say("Provisioning with Ansible...")

	ui.Message("Creating Ansible staging directory...")
	if err := p.createDir(ui, comm, p.config.StagingDir); err != nil {
		return fmt.Errorf("Error creating staging directory: %s", err)
	}

	ui.Message("Uploading main Playbook file...")
	if err := p.upload(ui, comm, p.config.PlaybookFile); err != nil {
		return fmt.Errorf("Error uploading main playbook: %s", err)
	}

	if len(p.config.RolePaths) > 0 {
		ui.Message("Uploading role directories...")
		for _, path := range p.config.RolePaths {
			targetPath := fmt.Sprintf("%s/roles", p.config.StagingDir)
			if err := p.uploadDirectory(ui, comm, targetPath, path); err != nil {
				return fmt.Errorf("Error uploading roles: %s", err)
			}
		}
	}

	if len(p.config.PlaybookPaths) > 0 {
		ui.Message("Uploading additional Playbooks...")
		for _, path := range p.config.PlaybookPaths {
			if err := p.upload(ui, comm, path); err != nil {
				return fmt.Errorf("Error uploading playbooks: %s", err)
			}
		}
	}

	if err := p.executeAnsible(ui, comm); err != nil {
		return fmt.Errorf("Error executing Ansible: %s", err)
	}

	return nil
}

func (p *Provisioner) Cancel() {
	// Just hard quit. It isn't a big deal if what we're doing keeps
	// running on the other side.
	os.Exit(0)
}

func (p *Provisioner) executeAnsible(ui packer.Ui, comm packer.Communicator) error {
	playbook := filepath.Join(p.config.StagingDir,
		filepath.Base(p.config.PlaybookFile))
	command := fmt.Sprintf("ansible-playbook %s -c local -i %s",
		playbook, `"127.0.0.1,"`)
	ui.Message(fmt.Sprintf("Executing Ansible: %s", command))
	cmd := &packer.RemoteCmd{
		Command: command,
	}
	if err := cmd.StartWithUi(comm, ui); err != nil {
		return err
	}
	if cmd.ExitStatus != 0 {
		return fmt.Errorf("Non-zero exit status: %d", cmd.ExitStatus)
	}
	return nil
}

func validateDirConfig(path string, config string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %s is invalid: %s", config, path, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s: %s must point to a directory", config, path)
	}
	return nil
}

func validateFileConfig(name string, config string, req bool) error {
	if req {
		if name == "" {
			return fmt.Errorf("%s must be specified.", config)
		}
	}
	info, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("%s: %s is invalid: %s", config, name, err)
	} else if info.IsDir() {
		return fmt.Errorf("%s: %s must point to a file", config, name)
	}
	return nil
}

func (p *Provisioner) upload(ui packer.Ui, comm packer.Communicator, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Error opening: %s", err)
	}
	defer f.Close()

	dst := filepath.Join(p.config.StagingDir, filepath.Base(src))
	if err = comm.Upload(dst, f); err != nil {
		return fmt.Errorf("Error uploading %s: %s", src, err)
	}
	return nil
}

func (p *Provisioner) createDir(ui packer.Ui, comm packer.Communicator, dir string) error {
	ui.Message(fmt.Sprintf("Creating directory: %s", dir))
	cmd := &packer.RemoteCmd{
		Command: fmt.Sprintf("mkdir -p '%s'", dir),
	}

	if err := cmd.StartWithUi(comm, ui); err != nil {
		return err
	}

	if cmd.ExitStatus != 0 {
		return fmt.Errorf("Non-zero exit status.")
	}

	return nil
}

func (p *Provisioner) uploadDirectory(ui packer.Ui, comm packer.Communicator, dst string, src string) error {
	if err := p.createDir(ui, comm, dst); err != nil {
		return err
	}

	// Make sure there is a trailing "/" so that the directory isn't
	// created on the other side.
	if src[len(src)-1] != '/' {
		src = src + "/"
	}

	return comm.UploadDir(dst, src, nil)
}
