package providers

// Provider defines the interface that all cluster providers must implement
type Provider interface {
	// CreateCluster creates a new cluster with the given name
	CreateCluster(name string) error
	
	// ListClusters returns a list of all clusters
	ListClusters() ([]string, error)
	
	// AddNode adds a new node to the specified cluster
	AddNode(clusterName, nodeName string) error
	
	// RemoveNode removes a node from the specified cluster
	RemoveNode(clusterName, nodeName string) error
	
	// DeleteCluster deletes the specified cluster
	DeleteCluster(name string) error
	
	// GetClusterStatus returns the status of the specified cluster
	GetClusterStatus(name string) (string, error)

	// GetKubeconfig returns the kubeconfig file path for the specified cluster
	GetKubeconfig(clusterName string) (string, error)
} 