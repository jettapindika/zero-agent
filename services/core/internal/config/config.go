package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ToolConfig struct {
	Permission      string   `json:"permission"`
	Timeout         int      `json:"timeout,omitempty"`
	AllowedCommands []string `json:"allowedCommands,omitempty"`
}

type AgentConfig struct {
	Model    string `json:"model,omitempty"`
	MaxSteps int    `json:"maxSteps,omitempty"`
}

type MCPEntry struct {
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
}

type UIConfig struct {
	Sidebar     bool `json:"sidebar"`
	DetailPanel bool `json:"detailPanel"`
}

type Config struct {
	Model  string                 `json:"model"`
	Theme  string                 `json:"theme"`
	Agents map[string]AgentConfig `json:"agents,omitempty"`
	Tools  map[string]ToolConfig  `json:"tools,omitempty"`
	MCP    map[string]MCPEntry    `json:"mcp,omitempty"`
	UI     UIConfig               `json:"ui"`
}

func Default() Config {
	return Config{
		Model: "anthropic/claude-sonnet-4-5",
		Theme: "tokyonight",
		Agents: map[string]AgentConfig{
			"build": {Model: "anthropic/claude-sonnet-4-5", MaxSteps: 100},
			"plan":  {Model: "anthropic/claude-haiku-4-5", MaxSteps: 50},
		},
		Tools: map[string]ToolConfig{
			"bash":  {Permission: "ask", Timeout: 30000},
			"write": {Permission: "ask"},
			"edit":  {Permission: "ask"},
			"read":  {Permission: "always"},
			"grep":  {Permission: "always"},
			"glob":  {Permission: "always"},
			"ls":    {Permission: "always"},
			"fetch": {Permission: "ask"},
		},
		UI: UIConfig{Sidebar: true, DetailPanel: true},
	}
}

func Load(projectPath string) (Config, error) {
	cfg := Default()

	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".zero", "config.json")
	mergeFromFile(&cfg, globalPath)

	if projectPath != "" {
		mergeFromFile(&cfg, filepath.Join(projectPath, "zero.json"))
		mergeFromFile(&cfg, filepath.Join(projectPath, ".zero", "config.json"))
	}

	return cfg, nil
}

func mergeFromFile(cfg *Config, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, cfg)
}
