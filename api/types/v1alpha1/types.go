package v1alpha1

type NodeConfig struct {
	Command []string `json:"command,omitempty"` // Command to pass into pod.
	Args    []string `json:"args,omitempty"`    // Command args to pass into the pod.
	Image   string   `json:"image,omitempty"`   // Docker image to use with pod.
	// Map of environment variables to pass into the pod.
	Env map[string]string `json:"env,omitempty"`
	// Specific entry point command for accessing the pod.
	EntryCommand string `json:"entry_command,omitempty"`
	// Mount point for configuration inside the pod.
	ConfigPath string `json:"config_path,omitempty"`
	// Default configuration file name for the pod.
	ConfigFile string `json:"config_file,omitempty"`
	Sleep      uint32 `json:"sleep,omitempty"` // Sleeptime before starting the pod.
}
