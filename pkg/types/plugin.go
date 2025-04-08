package types

// PluginType representa o tipo de plugin
type PluginType string

const (
	// PluginTypeCluster representa um plugin que afeta o cluster inteiro
	PluginTypeCluster PluginType = "cluster"
	// PluginTypeNodes representa um plugin que afeta nós específicos
	PluginTypeNodes PluginType = "nodes"
)

// AdapterType representa o tipo de adaptador
type AdapterType string

const (
	// AdapterTypeLocalDocker representa o adaptador Docker local
	AdapterTypeLocalDocker AdapterType = "localdocker"
	// AdapterTypeAWS representa o adaptador AWS
	AdapterTypeAWS AdapterType = "aws"
	// AdapterTypeIBM representa o adaptador IBM Cloud
	AdapterTypeIBM AdapterType = "ibm"
)

// Command representa um comando a ser executado
type Command struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args"`
}

// Plugin representa um plugin que pode ser instalado no cluster
type Plugin struct {
	Name           string       `yaml:"name"`
	Description    string       `yaml:"description"`
	Type           PluginType   `yaml:"type"`
	AllowedAdapters []AdapterType `yaml:"allowed_adapters"`
	ManifestURL    string       `yaml:"manifest_url"`
	Commands       []Command    `yaml:"commands"`
}

// PluginList representa uma lista de plugins
type PluginList struct {
	Plugins []Plugin `yaml:"plugins"`
} 