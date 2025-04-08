package cmd

import (
	"fmt"
	"github.com/andreixhz/ak3s/internal/core"
	"github.com/spf13/cobra"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage Kubernetes clusters",
	Long:  `Create, list, and manage Kubernetes clusters across different providers.`,
}

var createClusterCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager := core.NewClusterManager()
		err := manager.CreateCluster(args[0])
		if err != nil {
			fmt.Printf("Error creating cluster: %v\n", err)
			return
		}
		fmt.Printf("Cluster '%s' created successfully\n", args[0])
	},
}

var listClustersCmd = &cobra.Command{
	Use:   "list",
	Short: "List all clusters",
	Run: func(cmd *cobra.Command, args []string) {
		manager := core.NewClusterManager()
		clusters, err := manager.ListClusters()
		if err != nil {
			fmt.Printf("Error listing clusters: %v\n", err)
			return
		}
		
		if len(clusters) == 0 {
			fmt.Println("No clusters found")
			return
		}
		
		fmt.Println("Clusters:")
		for _, cluster := range clusters {
			status, _ := manager.GetClusterStatus(cluster)
			fmt.Printf("- %s (Status: %s)\n", cluster, status)
		}
	},
}

var deleteClusterCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager := core.NewClusterManager()
		err := manager.DeleteCluster(args[0])
		if err != nil {
			fmt.Printf("Error deleting cluster: %v\n", err)
			return
		}
		fmt.Printf("Cluster '%s' deleted successfully\n", args[0])
	},
}

var accessClusterCmd = &cobra.Command{
	Use:   "access [name]",
	Short: "Access a cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager := core.NewClusterManager()
		kubeconfigPath, err := manager.GetKubeconfig(args[0])
		if err != nil {
			fmt.Printf("Error accessing cluster: %v\n", err)
			return
		}
		fmt.Printf("Cluster accessed successfully. Kubeconfig saved to: %s\n", kubeconfigPath)
		fmt.Println("\nYou can now use kubectl to interact with the cluster. For example:")
		fmt.Println("kubectl get nodes")
		fmt.Println("kubectl get pods -A")
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
	clusterCmd.AddCommand(createClusterCmd)
	clusterCmd.AddCommand(listClustersCmd)
	clusterCmd.AddCommand(deleteClusterCmd)
	clusterCmd.AddCommand(accessClusterCmd)
} 