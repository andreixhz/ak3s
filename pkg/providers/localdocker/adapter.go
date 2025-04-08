package localdocker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	cmd := exec.Command("docker", "ps", "--filter", "ancestor=rancher/k3s:v1.32.3-k3s1", "--format", "{{.Names}}")
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
	// Get the token from the master node
	tokenCmd := exec.Command("docker", "exec", clusterName, "cat", "/var/lib/rancher/k3s/server/node-token")
	var tokenOut bytes.Buffer
	tokenCmd.Stdout = &tokenOut
	err := tokenCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to get token from master: %v", err)
	}
	token := strings.TrimSpace(tokenOut.String())

	// Get the master node IP
	ipCmd := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", clusterName)
	var ipOut bytes.Buffer
	ipCmd.Stdout = &ipOut
	err = ipCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to get master IP: %v", err)
	}
	masterIP := strings.TrimSpace(ipOut.String())

	// Create the worker node
	cmd := exec.Command("docker", "run", "-d",
		"--name", nodeName,
		"--privileged",
		"--tmpfs", "/run",
		"--tmpfs", "/var/run",
		"-e", "K3S_URL=https://"+masterIP+":6443",
		"-e", "K3S_TOKEN="+token,
		"rancher/k3s:v1.32.3-k3s1",
		"agent")
	return cmd.Run()
}

func (a *LocalDockerAdapter) RemoveNode(clusterName, nodeName string) error {
	// Get kubeconfig first
	kubeconfigPath, err := a.GetKubeconfig(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	// Get the actual node name from Kubernetes
	getNodeCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "-o", "name")
	var nodeOut bytes.Buffer
	getNodeCmd.Stdout = &nodeOut
	err = getNodeCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %v", err)
	}

	nodes := strings.Split(strings.TrimSpace(nodeOut.String()), "\n")
	found := false
	for _, node := range nodes {
		if strings.Contains(node, nodeName) {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("Node %s not found in Kubernetes cluster\n", nodeName)
	} else {
		// Drain the node first
		drainCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "drain", nodeName, "--ignore-daemonsets", "--delete-emptydir-data", "--force")
		err = drainCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to drain node: %v", err)
		}

		// Delete the node from Kubernetes
		deleteCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "node", nodeName)
		err = deleteCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to delete node from Kubernetes: %v", err)
		}
	}

	// Finally, remove the Docker container
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

func (a *LocalDockerAdapter) GetKubeconfig(clusterName string) (string, error) {
	// Get the kubeconfig from the master node
	cmd := exec.Command("docker", "exec", clusterName, "cat", "/etc/rancher/k3s/k3s.yaml")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	// Replace the server address with localhost:6443
	kubeconfig := strings.Replace(out.String(), "127.0.0.1", "localhost", -1)

	// Save the kubeconfig to a file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")
	err = os.MkdirAll(filepath.Dir(kubeconfigPath), 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create .kube directory: %v", err)
	}

	err = os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %v", err)
	}

	return kubeconfigPath, nil
}