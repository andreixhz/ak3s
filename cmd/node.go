package cmd

import (
	"fmt"
	"github.com/andreixhz/ak3s/internal/core"
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage nodes in a cluster",
	Long:  `Add and remove nodes from Kubernetes clusters.`,
}

var addNodeCmd = &cobra.Command{
	Use:   "add [cluster-name] [node-name]",
	Short: "Add a node to a cluster",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		manager := core.NewClusterManager()
		err := manager.AddNode(args[0], args[1])
		if err != nil {
			fmt.Printf("Error adding node: %v\n", err)
			return
		}
		fmt.Printf("Node '%s' added to cluster '%s' successfully\n", args[1], args[0])
	},
}

var removeNodeCmd = &cobra.Command{
	Use:   "remove [cluster-name] [node-name]",
	Short: "Remove a node from a cluster",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		manager := core.NewClusterManager()
		err := manager.RemoveNode(args[0], args[1])
		if err != nil {
			fmt.Printf("Error removing node: %v\n", err)
			return
		}
		fmt.Printf("Node '%s' removed from cluster '%s' successfully\n", args[1], args[0])
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(addNodeCmd)
	nodeCmd.AddCommand(removeNodeCmd)
} 