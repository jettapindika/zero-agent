package provider

import (
	"fmt"
	"strings"
)

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}}
}

func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) Provider {
	return r.providers[name]
}

func (r *Registry) ForModel(model string) (Provider, error) {
	prefix, _, ok := strings.Cut(model, "/")
	if !ok {
		return nil, fmt.Errorf("invalid model format: %s (expected provider/model)", model)
	}
	p := r.providers[prefix]
	if p == nil {
		return nil, fmt.Errorf("no provider registered for %q", prefix)
	}
	return p, nil
}
