package tool

import (
	"context"
	"encoding/json"
)

type Context struct {
	ProjectPath string
	SessionID   string
	MessageID   string
}

type Result struct {
	Title   string `json:"title"`
	Output  string `json:"output"`
	IsError bool   `json:"isError"`
}

type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	NeedsPermission() bool
	Execute(ctx context.Context, args json.RawMessage, tc Context) (Result, error)
}

type Registry struct {
	tools map[string]Tool
}

func DefaultRegistry() *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range []Tool{Read(), Ls(), Glob(), Grep(), Walk(), Bash(), Write(), Edit(), Fetch()} {
		r.tools[t.Name()] = t
	}
	return r
}

func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

func (r *Registry) Register(t Tool) {
	if r.tools == nil {
		r.tools = map[string]Tool{}
	}
	r.tools[t.Name()] = t
}
