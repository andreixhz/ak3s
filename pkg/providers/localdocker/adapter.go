package localdocker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// LocalDockerAdapter implements the providers.Provider interface
type LocalDockerAdapter struct{}

func (a *LocalDockerAdapter) CreateCluster(name string) error {
	// Create master node
	err := a.CreateMasterNode(name)
	if err != nil {
		return fmt.Errorf("failed to create master node: %v", err)
	}
	return nil
}

func (a *LocalDockerAdapter) CreateMasterNode(name string) error {
	cmd := exec.Command("docker", "run", "-d",
		"--name", name,
		"--privileged",
		"--tmpfs", "/run",
		"--tmpfs", "/var/run",
		"-e", "K3S_KUBECONFIG_MODE=644",
		"-e", "K3S_CLUSTER_INIT=true",
		"-p", "6443:6443",
		"-p", "80:80",
		"-p", "443:443",
		"-v", "/var/lib/rancher/k3s:/var/lib/rancher/k3s",
		"-v", "/etc/rancher/k3s:/etc/rancher/k3s",
		"rancher/k3s:v1.32.3-k3s1",
		"server",
		"--disable=traefik",
		"--disable=servicelb")
	return cmd.Run()
}

func (a *LocalDockerAdapter) ListClusters() ([]string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "name=ak3s-", "--format", "{{.Names}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	
	clusters := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(clusters) == 1 && clusters[0] == "" {
		return []string{}, nil
	}
	return clusters, nil
}

func (a *LocalDockerAdapter) AddNode(clusterName, nodeName string) error {
	cmd := exec.Command("docker", "run", "-d",
		"--name", nodeName,
		"--privileged",
		"--link", clusterName,
		"ak3s-worker")
	return cmd.Run()
}

func (a *LocalDockerAdapter) RemoveNode(clusterName, nodeName string) error {
	cmd := exec.Command("docker", "rm", "-f", nodeName)
	return cmd.Run()
}

func (a *LocalDockerAdapter) DeleteCluster(name string) error {
	cmd := exec.Command("docker", "rm", "-f", name)
	return cmd.Run()
}

func (a *LocalDockerAdapter) GetClusterStatus(name string) (string, error) {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", name)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}