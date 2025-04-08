package localdocker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
		"--flannel-backend=none",
		"--disable=traefik",
		"--disable=servicelb")
	
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create master node: %v", err)
	}

	// Wait for the cluster to be ready
	time.Sleep(30 * time.Second)

	// Get kubeconfig
	kubeconfigPath, err := a.GetKubeconfig(name)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %v", err)
	}

	// Install Calico CNI
	calicoCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/calico.yaml")
	err = calicoCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install Calico CNI: %v", err)
	}

	// Install MetalLB
	metallbCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml")
	err = metallbCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install MetalLB: %v", err)
	}

	// Wait for MetalLB to be ready
	time.Sleep(30 * time.Second)

	// Configure MetalLB IP pool
	metallbConfig := `apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - 172.18.255.200-172.18.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
  - default`

	metallbConfigFile, err := os.CreateTemp("", "metallb-config-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create MetalLB config file: %v", err)
	}
	defer os.Remove(metallbConfigFile.Name())

	_, err = metallbConfigFile.WriteString(metallbConfig)
	if err != nil {
		return fmt.Errorf("failed to write MetalLB config: %v", err)
	}
	metallbConfigFile.Close()

	applyMetallbConfig := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", metallbConfigFile.Name())
	err = applyMetallbConfig.Run()
	if err != nil {
		return fmt.Errorf("failed to apply MetalLB config: %v", err)
	}

	// Install NGINX Ingress Controller
	nginxCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.9.4/deploy/static/provider/cloud/deploy.yaml")
	err = nginxCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install NGINX Ingress Controller: %v", err)
	}

	return nil
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
		"-e", "K3S_NODE_NAME="+nodeName,
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
	kubernetesNodeName := ""
	for _, node := range nodes {
		nodeNameWithoutPrefix := strings.TrimPrefix(node, "node/")
		if nodeNameWithoutPrefix == nodeName {
			found = true
			kubernetesNodeName = nodeNameWithoutPrefix
			break
		}
	}

	if !found {
		fmt.Printf("Node %s not found in Kubernetes cluster\n", nodeName)
	} else {
		// Drain the node first
		drainCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "drain", kubernetesNodeName, "--ignore-daemonsets", "--delete-emptydir-data", "--force")
		err = drainCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to drain node: %v", err)
		}

		// Delete the node from Kubernetes
		deleteCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "node", kubernetesNodeName)
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