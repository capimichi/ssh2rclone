package model

// SSHEntry represents a parsed SSH host entry from ssh_config.
type SSHEntry struct {
	Alias        string
	HostName     string
	User         string
	Port         string
	KeyFile      string
	ProxyJump    string
	ProxyCommand string
	IsComplex    bool
}

// RcloneRemote represents an SFTP remote block to be written to rclone.conf.
type RcloneRemote struct {
	Name       string
	Type       string
	Host       string
	User       string
	Port       string
	KeyFile    string
	SSHCommand string
}

// SyncOptions holds CLI flags and runtime parameters.
type SyncOptions struct {
	SSHConfigPath    string
	RcloneConfigPath string
	ConfigPath       string
	NameTemplate     string
	DryRun           bool
	AllSSH           bool
	NoBackup         bool
	Verbose          bool
}
