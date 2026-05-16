package agent

// Agent defines a named agent with model, system prompt, and tool permissions.
type Agent struct {
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	SystemPrompt string   `json:"systemPrompt"`
	MaxSteps     int      `json:"maxSteps"`
	AllowedTools []string `json:"allowedTools,omitempty"`
	DeniedTools  []string `json:"deniedTools,omitempty"`
	ReadOnly     bool     `json:"readOnly"`
}

// Registry holds named agents.
type Registry struct {
	agents map[string]*Agent
}

func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]*Agent)}
}

func (r *Registry) Register(a *Agent) {
	r.agents[a.Name] = a
}

func (r *Registry) Get(name string) *Agent {
	return r.agents[name]
}

func (r *Registry) List() []*Agent {
	out := make([]*Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out
}

// DefaultAgents returns the built-in agent set like OpenCode.
func DefaultAgents(defaultModel string) []*Agent {
	if defaultModel == "" {
		defaultModel = "cx/gpt-5.5"
	}
	return []*Agent{
		{
			Name:         "build",
			Model:        defaultModel,
			SystemPrompt: "You are Zero, an AI coding agent. You can read files, search code, run commands, and edit files to help the user with their coding tasks. Be concise and helpful.",
			MaxSteps:     100,
			AllowedTools: []string{"read", "ls", "glob", "grep", "bash", "write", "edit", "fetch"},
			ReadOnly:     false,
		},
		{
			Name:         "plan",
			Model:        defaultModel,
			SystemPrompt: "You are Zero in planning mode. You can read files and search code to understand the codebase and produce implementation plans. You cannot write files or run destructive commands.",
			MaxSteps:     50,
			AllowedTools: []string{"read", "ls", "glob", "grep"},
			DeniedTools:  []string{"bash", "write", "edit"},
			ReadOnly:     true,
		},
		{
			Name:         "explore",
			Model:        defaultModel,
			SystemPrompt: "You are Zero in explore mode. A fast, read-only agent for exploring codebases. You cannot modify files. Use this to quickly find files by patterns, search code for keywords, or answer questions about the codebase.",
			MaxSteps:     30,
			AllowedTools: []string{"read", "ls", "glob", "grep"},
			DeniedTools:  []string{"bash", "write", "edit", "fetch"},
			ReadOnly:     true,
		},
	}
}
