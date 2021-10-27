package v1alpha1

const (
	defaultSRLinuxImageName = "ghcr.io/nokia/srlinux"
	defaultSrlinuxVariant   = "ixrd2"
)

var (
	defaultCmd = []string{
		"/tini",
		"--",
		"fixuid",
		"-q",
		"/kne-entrypoint.sh",
	}

	defaultArgs = []string{
		"sudo",
		"bash",
		"-c",
		"touch /.dockerenv && /opt/srlinux/bin/sr_linux",
	}

	defaultConstraints = map[string]string{
		"cpu":    "0.5",
		"memory": "1Gi",
	}
)

type NodeConfig struct {
	Command []string `json:"command,omitempty"` // Command to pass into pod.
	Args    []string `json:"args,omitempty"`    // Command args to pass into the pod.
	Image   string   `json:"image,omitempty"`   // Docker image to use with pod.
	// Map of environment variables to pass into the pod.
	Env map[string]string `json:"env,omitempty"`
	// Specific entry point command for accessing the pod.
	EntryCommand string `json:"entry_command,omitempty"`
	// Mount point for configuration inside the pod. Should point to a dir that contains ConfigFile
	ConfigPath string `json:"config_path,omitempty"`
	// Startup configuration file name for the pod. Set in the kne topo and created by kne as a config map
	ConfigFile string `json:"config_file,omitempty"`
	// When set to true by kne, srlinux controller will attempt to mount the file with startup config to the pod
	ConfigDataPresent bool            `json:"config_data_present,omitempty"`
	Cert              *CertificateCfg `json:"cert,omitempty"`
	Sleep             uint32          `json:"sleep,omitempty"` // Sleeptime before starting the pod.
}

type CertificateCfg struct {
	// Certificate name on the node.
	CertName string `json:"cert_name,omitempty"`
	// Key name on the node.
	KeyName string `json:"key_name,omitempty"`
	// RSA keysize to use for key generation.
	KeySize uint32 `json:"key_size,omitempty"`
	// Common name to set in the cert.
	CommonName string `json:"common_name,omitempty"`
}

func (n *NodeConfig) GetCommand() []string {
	if n.Command != nil {
		return n.Command
	}

	return defaultCmd
}

func (n *NodeConfig) GetArgs() []string {
	if n.Args != nil {
		return n.Args
	}

	return defaultArgs
}
