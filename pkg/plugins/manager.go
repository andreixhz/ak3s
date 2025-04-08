package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andreixhz/ak3s/pkg/types"
	"gopkg.in/yaml.v3"
)

// PluginManager gerencia os plugins do sistema
type PluginManager struct {
	plugins []types.Plugin
}

// NewPluginManager cria uma nova instância do PluginManager
func NewPluginManager() (*PluginManager, error) {
	pm := &PluginManager{}
	
	// Carrega os plugins do arquivo YAML
	err := pm.loadPlugins()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugins: %v", err)
	}

	return pm, nil
}

// loadPlugins carrega os plugins do arquivo YAML
func (pm *PluginManager) loadPlugins() error {
	// Tenta encontrar o arquivo plugins.yaml em diferentes locais
	possiblePaths := []string{
		"plugins.yaml",
		"/etc/ak3s/plugins.yaml",
		filepath.Join(os.Getenv("HOME"), ".ak3s", "plugins.yaml"),
	}

	var pluginList types.PluginList
	var found bool

	for _, path := range possiblePaths {
		data, err := os.ReadFile(path)
		if err == nil {
			err = yaml.Unmarshal(data, &pluginList)
			if err == nil {
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("no plugins.yaml file found in any of the expected locations")
	}

	pm.plugins = pluginList.Plugins
	return nil
}

// GetPlugins retorna todos os plugins carregados
func (pm *PluginManager) GetPlugins() []types.Plugin {
	return pm.plugins
}

// GetPluginByName retorna um plugin pelo nome
func (pm *PluginManager) GetPluginByName(name string) (*types.Plugin, error) {
	for _, plugin := range pm.plugins {
		if plugin.Name == name {
			return &plugin, nil
		}
	}
	return nil, fmt.Errorf("plugin %s not found", name)
}

// GetPluginsByType retorna todos os plugins de um tipo específico
func (pm *PluginManager) GetPluginsByType(pluginType types.PluginType) []types.Plugin {
	var result []types.Plugin
	for _, plugin := range pm.plugins {
		if plugin.Type == pluginType {
			result = append(result, plugin)
		}
	}
	return result
}

// GetPluginsByAdapter retorna todos os plugins compatíveis com um adaptador específico
func (pm *PluginManager) GetPluginsByAdapter(adapterType types.AdapterType) []types.Plugin {
	var result []types.Plugin
	for _, plugin := range pm.plugins {
		for _, allowedAdapter := range plugin.AllowedAdapters {
			if allowedAdapter == adapterType {
				result = append(result, plugin)
				break
			}
		}
	}
	return result
}

// IsPluginCompatible verifica se um plugin é compatível com um adaptador específico
func (pm *PluginManager) IsPluginCompatible(pluginName string, adapterType types.AdapterType) bool {
	plugin, err := pm.GetPluginByName(pluginName)
	if err != nil {
		return false
	}

	for _, allowedAdapter := range plugin.AllowedAdapters {
		if allowedAdapter == adapterType {
			return true
		}
	}

	return false
} 