package olt

import "context"

// Adapter isolates vendor-specific OLT protocols from the core NMS.
type Adapter interface {
	Discover(ctx context.Context, olt OLT) ([]PONPort, error)
	DiscoverONUs(ctx context.Context, olt OLT, port PONPort) ([]ONU, error)
	PollONU(ctx context.Context, olt OLT, onu ONU) (ONU, error)
}

// Registry selects an adapter by normalized vendor name.
type Registry struct { adapters map[string]Adapter }

func NewRegistry() *Registry { return &Registry{adapters: make(map[string]Adapter)} }
func (r *Registry) Register(vendor string, adapter Adapter) { r.adapters[vendor] = adapter }
func (r *Registry) Get(vendor string) (Adapter, bool) { a, ok := r.adapters[vendor]; return a, ok }
