package localdocker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/andreixhz/ak3s/pkg/progress"
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
	p := progress.NewProgress(6)
	p.Update("Creating master node container...")

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
		p.Error(fmt.Errorf("failed to create master node: %v", err))
	}

	p.Update("Waiting for cluster to be ready...")
	// Wait for the cluster to be ready
	for i := 0; i < 30; i++ {
		statusCmd := exec.Command("docker", "exec", name, "kubectl", "get", "nodes")
		err = statusCmd.Run()
		if err == nil {
			break
		}
		time.Sleep(10 * time.Second)
	}
	if err != nil {
		p.Error(fmt.Errorf("cluster failed to become ready: %v", err))
	}

	p.Update("Getting kubeconfig...")
	kubeconfigPath, err := a.GetKubeconfig(name)
	if err != nil {
		p.Error(fmt.Errorf("failed to get kubeconfig: %v", err))
	}

	p.Update("Installing Calico CNI...")
	calicoCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/calico.yaml")
	err = calicoCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to install Calico CNI: %v", err))
	}

	// Wait for Calico to be ready
	p.Update("Waiting for Calico to be ready...")
	for i := 0; i < 30; i++ {
		statusCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "pods", "-n", "kube-system", "-l", "k8s-app=calico-node", "-o", "jsonpath='{.items[*].status.phase}'")
		var statusOut bytes.Buffer
		statusCmd.Stdout = &statusOut
		err = statusCmd.Run()
		if err == nil && strings.Contains(statusOut.String(), "Running") {
			break
		}
		time.Sleep(10 * time.Second)
	}
	if err != nil {
		p.Error(fmt.Errorf("Calico failed to become ready: %v", err))
	}

	p.Update("Installing MetalLB...")
	metallbCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml")
	err = metallbCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to install MetalLB: %v", err))
	}

	// Wait for MetalLB to be ready
	p.Update("Waiting for MetalLB to be ready...")
	for i := 0; i < 30; i++ {
		statusCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "pods", "-n", "metallb-system", "-o", "jsonpath='{.items[*].status.phase}'")
		var statusOut bytes.Buffer
		statusCmd.Stdout = &statusOut
		err = statusCmd.Run()
		if err == nil && strings.Contains(statusOut.String(), "Running") {
			break
		}
		time.Sleep(10 * time.Second)
	}
	if err != nil {
		p.Error(fmt.Errorf("MetalLB failed to become ready: %v", err))
	}

	p.Update("Configuring MetalLB...")
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
		p.Error(fmt.Errorf("failed to create MetalLB config file: %v", err))
	}
	defer os.Remove(metallbConfigFile.Name())

	_, err = metallbConfigFile.WriteString(metallbConfig)
	if err != nil {
		p.Error(fmt.Errorf("failed to write MetalLB config: %v", err))
	}
	metallbConfigFile.Close()

	applyMetallbConfig := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", metallbConfigFile.Name())
	err = applyMetallbConfig.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to apply MetalLB config: %v", err))
	}

	p.Update("Installing NGINX Ingress Controller...")
	nginxCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.9.4/deploy/static/provider/cloud/deploy.yaml")
	err = nginxCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to install NGINX Ingress Controller: %v", err))
	}

	p.Success()
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
	p := progress.NewProgress(4)
	p.Update("Getting token from master node...")

	tokenCmd := exec.Command("docker", "exec", clusterName, "cat", "/var/lib/rancher/k3s/server/node-token")
	var tokenOut bytes.Buffer
	tokenCmd.Stdout = &tokenOut
	err := tokenCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to get token from master: %v", err))
	}
	token := strings.TrimSpace(tokenOut.String())

	p.Update("Getting master node IP...")
	ipCmd := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", clusterName)
	var ipOut bytes.Buffer
	ipCmd.Stdout = &ipOut
	err = ipCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to get master IP: %v", err))
	}
	masterIP := strings.TrimSpace(ipOut.String())

	p.Update("Creating worker node container...")
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
	err = cmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to create worker node: %v", err))
	}

	p.Update("Waiting for node to join the cluster...")
	time.Sleep(30 * time.Second)

	p.Success()
	return nil
}

func (a *LocalDockerAdapter) RemoveNode(clusterName, nodeName string) error {
	p := progress.NewProgress(4)
	p.Update("Getting kubeconfig...")

	kubeconfigPath, err := a.GetKubeconfig(clusterName)
	if err != nil {
		p.Error(fmt.Errorf("failed to get kubeconfig: %v", err))
	}

	p.Update("Checking if node exists in Kubernetes...")
	getNodeCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "-o", "name")
	var nodeOut bytes.Buffer
	getNodeCmd.Stdout = &nodeOut
	err = getNodeCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to get nodes: %v", err))
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
		p.Update(fmt.Sprintf("Node %s not found in Kubernetes cluster", nodeName))
	} else {
		p.Update("Draining node from Kubernetes...")
		drainCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "drain", kubernetesNodeName, "--ignore-daemonsets", "--delete-emptydir-data", "--force")
		err = drainCmd.Run()
		if err != nil {
			p.Error(fmt.Errorf("failed to drain node: %v", err))
		}

		p.Update("Removing node from Kubernetes...")
		deleteCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "node", kubernetesNodeName)
		err = deleteCmd.Run()
		if err != nil {
			p.Error(fmt.Errorf("failed to delete node from Kubernetes: %v", err))
		}
	}

	p.Update("Removing Docker container...")
	cmd := exec.Command("docker", "rm", "-f", nodeName)
	err = cmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to remove Docker container: %v", err))
	}

	p.Success()
	return nil
}

func (a *LocalDockerAdapter) DeleteCluster(name string) error {
	p := progress.NewProgress(7)
	p.Update("Getting kubeconfig...")

	kubeconfigPath, err := a.GetKubeconfig(name)
	if err != nil {
		p.Error(fmt.Errorf("failed to get kubeconfig: %v", err))
	}

	p.Update("Removing all resources from the cluster...")
	// Get all namespaces except system ones
	namespacesCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "namespaces", "-o", "name")
	var namespacesOut bytes.Buffer
	namespacesCmd.Stdout = &namespacesOut
	err = namespacesCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to get namespaces: %v", err))
	}

	namespaces := strings.Split(strings.TrimSpace(namespacesOut.String()), "\n")
	for _, ns := range namespaces {
		if ns == "" || strings.Contains(ns, "kube-system") || strings.Contains(ns, "kube-public") || strings.Contains(ns, "kube-node-lease") {
			continue
		}
		namespace := strings.TrimPrefix(ns, "namespace/")
		
		// Delete all resources in the namespace
		deleteCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "all", "--all", "-n", namespace, "--force", "--grace-period=0")
		var deleteOut bytes.Buffer
		deleteCmd.Stdout = &deleteOut
		deleteCmd.Stderr = &deleteOut
		deleteCmd.Run()
	}

	// Delete all CRDs
	p.Update("Removing all CRDs...")
	crdsCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "crds", "-o", "name")
	var crdsOut bytes.Buffer
	crdsCmd.Stdout = &crdsOut
	err = crdsCmd.Run()
	if err == nil {
		crds := strings.Split(strings.TrimSpace(crdsOut.String()), "\n")
		for _, crd := range crds {
			if crd == "" {
				continue
			}
			deleteCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", crd, "--all", "--force", "--grace-period=0")
			deleteCmd.Run()
		}
	}

	p.Update("Getting all nodes in the cluster...")
	getNodesCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "-o", "name")
	var nodesOut bytes.Buffer
	getNodesCmd.Stdout = &nodesOut
	err = getNodesCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to get nodes: %v", err))
	}

	p.Update("Draining all nodes...")
	nodes := strings.Split(strings.TrimSpace(nodesOut.String()), "\n")
	for _, node := range nodes {
		if node == "" {
			continue
		}
		nodeName := strings.TrimPrefix(node, "node/")
		
		// Drain the node with all options to ensure complete cleanup
		drainCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "drain", nodeName,
			"--ignore-daemonsets",
			"--delete-emptydir-data",
			"--force",
			"--grace-period=0",
			"--timeout=5m",
			"--disable-eviction=true")
		var drainOut bytes.Buffer
		drainCmd.Stdout = &drainOut
		drainCmd.Stderr = &drainOut
		err = drainCmd.Run()
		if err != nil {
			fmt.Printf("Warning: Drain of node %s had issues: %v\nOutput: %s\n", nodeName, err, drainOut.String())
		}

		// Wait for all pods to be evicted
		for i := 0; i < 30; i++ {
			podsCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "pods", "--all-namespaces", "-o", "wide", "--field-selector", "spec.nodeName="+nodeName)
			var podsOut bytes.Buffer
			podsCmd.Stdout = &podsOut
			podsCmd.Run()
			if strings.TrimSpace(podsOut.String()) == "" {
				break
			}
			time.Sleep(10 * time.Second)
		}

		// Delete the node from Kubernetes
		deleteCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "node", nodeName)
		err = deleteCmd.Run()
		if err != nil {
			fmt.Printf("Warning: Failed to delete node %s from Kubernetes: %v\n", nodeName, err)
		}

		// Remove the Docker container
		rmCmd := exec.Command("docker", "rm", "-f", nodeName)
		err = rmCmd.Run()
		if err != nil {
			fmt.Printf("Warning: Failed to remove Docker container %s: %v\n", nodeName, err)
		}
	}

	p.Update("Removing master node container...")
	cmd := exec.Command("docker", "rm", "-f", name)
	err = cmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to remove master node: %v", err))
	}

	p.Update("Cleaning up configuration files...")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		p.Error(fmt.Errorf("failed to get home directory: %v", err))
	}
	kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	os.Remove(kubeconfigPath)

	// Remove k3s data
	os.RemoveAll("/var/lib/rancher/k3s")
	os.RemoveAll("/etc/rancher/k3s")

	// Force remove all containers that might be related to this cluster
	forceRemoveCmd := exec.Command("docker", "ps", "-a", "--filter", "name="+name, "-q")
	var forceRemoveOut bytes.Buffer
	forceRemoveCmd.Stdout = &forceRemoveOut
	forceRemoveCmd.Run()
	containerIDs := strings.Split(strings.TrimSpace(forceRemoveOut.String()), "\n")
	for _, id := range containerIDs {
		if id != "" {
			exec.Command("docker", "rm", "-f", id).Run()
		}
	}

	p.Update("Removing Docker volumes...")
	// Get all volumes associated with the cluster
	volumeCmd := exec.Command("docker", "volume", "ls", "-q", "--filter", "name="+name)
	var volumeOut bytes.Buffer
	volumeCmd.Stdout = &volumeOut
	volumeCmd.Run()
	volumeIDs := strings.Split(strings.TrimSpace(volumeOut.String()), "\n")
	for _, id := range volumeIDs {
		if id != "" {
			exec.Command("docker", "volume", "rm", "-f", id).Run()
		}
	}

	// Also remove any volumes that might be mounted in the containers
	containerVolumeCmd := exec.Command("docker", "ps", "-a", "--filter", "name="+name, "--format", "{{.Mounts}}")
	var containerVolumeOut bytes.Buffer
	containerVolumeCmd.Stdout = &containerVolumeOut
	containerVolumeCmd.Run()
	containerVolumes := strings.Split(strings.TrimSpace(containerVolumeOut.String()), "\n")
	for _, volume := range containerVolumes {
		if volume != "" {
			// Extract volume name from mount string
			parts := strings.Split(volume, " ")
			if len(parts) > 0 {
				volumeName := parts[0]
				exec.Command("docker", "volume", "rm", "-f", volumeName).Run()
			}
		}
	}

	p.Success()
	return nil
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