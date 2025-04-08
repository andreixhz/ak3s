package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ak3s",
	Short: "AK3S - A simple Kubernetes cluster manager",
	Long: `AK3S is a CLI tool for managing Kubernetes clusters across different providers.
Currently supports local Docker provider, with plans to support AWS, IBM Cloud, and others.`,
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}