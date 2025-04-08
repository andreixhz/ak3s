package core

import (
	"github.com/andreixhz/ak3s/pkg/providers"
	"github.com/andreixhz/ak3s/pkg/providers/localdocker"
)

type ClusterManager struct {
	provider providers.Provider
}

func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		provider: &localdocker.LocalDockerAdapter{},
	}
}

func (m *ClusterManager) CreateCluster(name string) error {
	return m.provider.CreateCluster(name)
}

func (m *ClusterManager) ListClusters() ([]string, error) {
	return m.provider.ListClusters()
}

func (m *ClusterManager) AddNode(clusterName, nodeName string) error {
	return m.provider.AddNode(clusterName, nodeName)
}

func (m *ClusterManager) RemoveNode(clusterName, nodeName string) error {
	return m.provider.RemoveNode(clusterName, nodeName)
}

func (m *ClusterManager) DeleteCluster(name string) error {
	return m.provider.DeleteCluster(name)
}

func (m *ClusterManager) GetClusterStatus(name string) (string, error) {
	return m.provider.GetClusterStatus(name)
}

func (m *ClusterManager) GetKubeconfig(name string) (string, error) {
	return m.provider.GetKubeconfig(name)
}