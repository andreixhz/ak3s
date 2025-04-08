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
		"-e", "K3S_NODE_NAME="+name,
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
	// Install Calico using the official manifest with custom settings
	calicoManifest := `apiVersion: v1
kind: Namespace
metadata:
  name: calico-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: calico-node
  namespace: calico-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: calico-kube-controllers
  namespace: calico-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: calico-node
rules:
  - apiGroups: [""]
    resources:
      - namespaces
      - serviceaccounts
      - nodes
      - nodes/status
      - pods
      - services
      - endpoints
      - configmaps
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups: [""]
    resources:
      - configmaps
    resourceNames:
      - kubeadm-config
    verbs:
      - get
      - list
      - watch
  - apiGroups: ["networking.k8s.io"]
    resources:
      - networkpolicies
    verbs:
      - get
      - list
      - watch
  - apiGroups: ["crd.projectcalico.org"]
    resources:
      - globalfelixconfigs
      - felixconfigurations
      - bgppeers
      - globalbgpconfigs
      - bgpconfigurations
      - ippools
      - ipamblocks
      - globalnetworkpolicies
      - globalnetworksets
      - networkpolicies
      - networksets
      - clusterinformations
      - hostendpoints
      - blockaffinities
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: calico-node
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: calico-node
subjects:
- kind: ServiceAccount
  name: calico-node
  namespace: calico-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: calico-config
  namespace: calico-system
data:
  typha_service_name: "none"
  calico_backend: "bird"
  veth_mtu: "1440"
  cni_network_config: |-
    {
      "name": "k8s-pod-network",
      "cniVersion": "0.3.1",
      "plugins": [
        {
          "type": "calico",
          "log_level": "info",
          "log_file_path": "/var/log/calico/cni/cni.log",
          "datastore_type": "kubernetes",
          "nodename": "__KUBERNETES_NODE_NAME__",
          "mtu": __CNI_MTU__,
          "ipam": {
              "type": "calico-ipam"
          },
          "policy": {
              "type": "k8s"
          },
          "kubernetes": {
              "kubeconfig": "__KUBECONFIG_FILEPATH__"
          }
        },
        {
          "type": "portmap",
          "snat": true,
          "capabilities": {"portMappings": true}
        },
        {
          "type": "bandwidth",
          "capabilities": {"bandwidth": true}
        }
      ]
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: calico-node
  namespace: calico-system
  labels:
    k8s-app: calico-node
spec:
  selector:
    matchLabels:
      k8s-app: calico-node
  template:
    metadata:
      labels:
        k8s-app: calico-node
    spec:
      nodeSelector:
        kubernetes.io/os: linux
      hostNetwork: true
      tolerations:
        - effect: NoSchedule
          operator: Exists
        - key: CriticalAddonsOnly
          operator: Exists
      serviceAccountName: calico-node
      containers:
        - name: calico-node
          image: docker.io/calico/node:v3.26.1
          env:
            - name: DATASTORE_TYPE
              value: "kubernetes"
            - name: WAIT_FOR_DATASTORE
              value: "true"
            - name: NODENAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: CALICO_NETWORKING_BACKEND
              value: "bird"
            - name: CLUSTER_TYPE
              value: "k8s,bgp"
            - name: IP
              value: "autodetect"
            - name: IP_AUTODETECTION_METHOD
              value: "first-found"
            - name: CALICO_IPV4POOL_CIDR
              value: "10.42.0.0/16"
            - name: CALICO_IPV4POOL_IPIP
              value: "Always"
            - name: FELIX_IPINIPMTU
              value: "1440"
            - name: CALICO_DISABLE_FILE_LOGGING
              value: "true"
            - name: FELIX_DEFAULTENDPOINTTOHOSTACTION
              value: "ACCEPT"
            - name: FELIX_IPV6SUPPORT
              value: "false"
            - name: FELIX_LOGSEVERITYSCREEN
              value: "info"
            - name: FELIX_HEALTHENABLED
              value: "true"
          securityContext:
            privileged: true
          resources:
            requests:
              cpu: 250m
          livenessProbe:
            exec:
              command:
              - /bin/calico-node
              - -felix-live
              - -bird-live
            periodSeconds: 10
            initialDelaySeconds: 10
            failureThreshold: 6
          readinessProbe:
            exec:
              command:
              - /bin/calico-node
              - -felix-ready
              - -bird-ready
            periodSeconds: 10
          volumeMounts:
            - mountPath: /host/etc/cni/net.d
              name: cni-net-dir
            - mountPath: /var/lib/calico
              name: var-lib-calico
            - mountPath: /var/log/calico
              name: var-log-calico
            - mountPath: /var/run/calico
              name: var-run-calico
            - mountPath: /sys/fs/cgroup
              name: cgroup
              readOnly: true
      volumes:
        - name: cni-net-dir
          hostPath:
            path: /etc/cni/net.d
        - name: var-lib-calico
          hostPath:
            path: /var/lib/calico
        - name: var-log-calico
          hostPath:
            path: /var/log/calico
        - name: var-run-calico
          hostPath:
            path: /var/run/calico
        - name: policysync
          hostPath:
            path: /var/run/nodeagent
        - name: cgroup
          hostPath:
            path: /sys/fs/cgroup
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: calico-kube-controllers
  namespace: calico-system
  labels:
    k8s-app: calico-kube-controllers
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: calico-kube-controllers
  template:
    metadata:
      labels:
        k8s-app: calico-kube-controllers
    spec:
      nodeSelector:
        kubernetes.io/os: linux
      tolerations:
        - key: CriticalAddonsOnly
          operator: Exists
        - effect: NoSchedule
          operator: Exists
      serviceAccountName: calico-kube-controllers
      containers:
        - name: calico-kube-controllers
          image: docker.io/calico/kube-controllers:v3.26.1
          env:
            - name: DATASTORE_TYPE
              value: "kubernetes"
          readinessProbe:
            exec:
              command:
              - /usr/bin/check-status
              - -r
          resources:
            requests:
              cpu: 100m
              memory: 256Mi`

	// Create temporary file for Calico manifest
	calicoFile, err := os.CreateTemp("", "calico-*.yaml")
	if err != nil {
		p.Error(fmt.Errorf("failed to create Calico manifest file: %v", err))
	}
	defer os.Remove(calicoFile.Name())

	_, err = calicoFile.WriteString(calicoManifest)
	if err != nil {
		p.Error(fmt.Errorf("failed to write Calico manifest: %v", err))
	}
	calicoFile.Close()

	// Apply Calico manifest
	calicoCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", calicoFile.Name())
	err = calicoCmd.Run()
	if err != nil {
		p.Error(fmt.Errorf("failed to install Calico CNI: %v", err))
	}

	// Wait for Calico to be ready
	p.Update("Waiting for Calico to be ready...")
	for i := 0; i < 30; i++ {
		statusCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "pods", "-n", "calico-system", "-o", "jsonpath='{.items[*].status.phase}'")
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
	for _, node := range nodes {
		nodeNameWithoutPrefix := strings.TrimPrefix(node, "node/")
		if nodeNameWithoutPrefix == nodeName {
			found = true
			break
		}
	}

	if !found {
		p.Update(fmt.Sprintf("Node %s not found in Kubernetes cluster", nodeName))
	} else {
		p.Update("Draining node from Kubernetes...")
		drainCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "drain", nodeName, "--ignore-daemonsets", "--delete-emptydir-data", "--force")
		err = drainCmd.Run()
		if err != nil {
			p.Error(fmt.Errorf("failed to drain node: %v", err))
		}

		p.Update("Removing node from Kubernetes...")
		deleteCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "node", nodeName)
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
	p := progress.NewProgress(5)
	p.Update("Getting kubeconfig...")

	kubeconfigPath, err := a.GetKubeconfig(name)
	if err != nil {
		p.Error(fmt.Errorf("failed to get kubeconfig: %v", err))
	}

	p.Update("Removing all resources from the cluster...")
	// Delete all resources in all namespaces
	deleteAllCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "all", "--all", "--all-namespaces", "--force", "--grace-period=0")
	deleteAllCmd.Run()

	// Delete all CRDs
	deleteCRDsCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "delete", "crds", "--all", "--force", "--grace-period=0")
	deleteCRDsCmd.Run()

	p.Update("Stopping and removing all containers...")
	// Stop all containers first
	stopCmd := exec.Command("docker", "stop", "$(docker ps -a -q)")
	stopCmd.Run()

	// Remove all containers
	rmCmd := exec.Command("docker", "rm", "-f", "$(docker ps -a -q)")
	rmCmd.Run()

	// Remove all containers related to this cluster
	clusterContainersCmd := exec.Command("docker", "ps", "-a", "--filter", "name="+name, "-q")
	var clusterContainersOut bytes.Buffer
	clusterContainersCmd.Stdout = &clusterContainersOut
	clusterContainersCmd.Run()
	containerIDs := strings.Split(strings.TrimSpace(clusterContainersOut.String()), "\n")
	for _, id := range containerIDs {
		if id != "" {
			exec.Command("docker", "rm", "-f", id).Run()
		}
	}

	p.Update("Removing all Docker volumes and networks...")
	// Remove all volumes
	volumeCmd := exec.Command("docker", "volume", "prune", "-f")
	volumeCmd.Run()

	// Remove all networks
	networkCmd := exec.Command("docker", "network", "prune", "-f")
	networkCmd.Run()

	// Remove specific volumes associated with the cluster
	clusterVolumeCmd := exec.Command("docker", "volume", "ls", "-q", "--filter", "name="+name)
	var clusterVolumeOut bytes.Buffer
	clusterVolumeCmd.Stdout = &clusterVolumeOut
	clusterVolumeCmd.Run()
	volumeIDs := strings.Split(strings.TrimSpace(clusterVolumeOut.String()), "\n")
	for _, id := range volumeIDs {
		if id != "" {
			exec.Command("docker", "volume", "rm", "-f", id).Run()
		}
	}

	p.Update("Cleaning up configuration files...")
	// Remove kubeconfig
	homeDir, err := os.UserHomeDir()
	if err != nil {
		p.Error(fmt.Errorf("failed to get home directory: %v", err))
	}
	kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	os.Remove(kubeconfigPath)

	// Remove all k3s data
	os.RemoveAll("/var/lib/rancher/k3s")
	os.RemoveAll("/etc/rancher/k3s")
	os.RemoveAll("/var/lib/kubelet")
	os.RemoveAll("/var/lib/cni")
	os.RemoveAll("/var/log/containers")
	os.RemoveAll("/var/log/pods")
	os.RemoveAll("/var/log/k3s")

	// Remove Docker data
	os.RemoveAll("/var/lib/docker/containers")
	os.RemoveAll("/var/lib/docker/volumes")
	os.RemoveAll("/var/lib/docker/network")

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