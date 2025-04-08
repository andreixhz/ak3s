package plugins

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/andreixhz/ak3s/pkg/types"
)

// CommandExecutor executa comandos de plugins
type CommandExecutor struct {
	kubeconfig string
}

// NewCommandExecutor cria uma nova instância do CommandExecutor
func NewCommandExecutor(kubeconfig string) *CommandExecutor {
	return &CommandExecutor{
		kubeconfig: kubeconfig,
	}
}

// ExecuteCommand executa um comando de plugin
func (ce *CommandExecutor) ExecuteCommand(command types.Command, stdin string) error {
	// Cria o comando
	cmd := exec.Command(command.Command, command.Args...)

	// Configura o ambiente
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", ce.kubeconfig))

	// Se houver stdin, configura
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Captura a saída
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Executa o comando
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command %s: %v\nstdout: %s\nstderr: %s",
			command.Name, err, stdout.String(), stderr.String())
	}

	return nil
}

// ExecutePlugin executa todos os comandos de um plugin
func (ce *CommandExecutor) ExecutePlugin(plugin types.Plugin) error {
	for _, command := range plugin.Commands {
		// Verifica se o comando tem stdin
		var stdin string
		if command.Name == "configure" && plugin.Name == "metallb" {
			stdin = `apiVersion: metallb.io/v1beta1
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
		}

		err := ce.ExecuteCommand(command, stdin)
		if err != nil {
			return fmt.Errorf("failed to execute plugin %s: %v", plugin.Name, err)
		}
	}

	return nil
} 