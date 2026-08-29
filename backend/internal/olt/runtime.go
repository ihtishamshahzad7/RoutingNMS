package olt

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// RuntimeManager owns the lifecycle of one polling goroutine per enabled OLT.
type RuntimeManager struct {
	Config ConfigService
	Repo Repository
	mu sync.Mutex
	running map[string]context.CancelFunc
}

func NewRuntimeManager(config ConfigService, repo Repository) *RuntimeManager {
	return &RuntimeManager{Config: config, Repo: repo, running: make(map[string]context.CancelFunc)}
}

// Start loads enabled OLTs and starts a poller for each valid configuration.
// One bad OLT does not prevent other OLTs from being monitored.
func (m *RuntimeManager) Start(ctx context.Context) error {
	configs, err := m.Config.LoadEnabled(ctx)
	if err != nil { return err }
	for _, cfg := range configs {
		if err := m.startOne(ctx, cfg); err != nil { log.Printf("OLT runtime start failed id=%s: %v", cfg.OLT.ID, err) }
	}
	return nil
}

func (m *RuntimeManager) startOne(parent context.Context, cfg ConfiguredOLT) error {
	adapter, err := m.Config.AdapterFor(cfg)
	if err != nil { return err }
	m.mu.Lock()
	if _, exists := m.running[cfg.OLT.ID]; exists { m.mu.Unlock(); return nil }
	ctx, cancel := context.WithCancel(parent)
	m.running[cfg.OLT.ID] = cancel
	m.mu.Unlock()
	poller := Poller{Adapter: adapter, OLT: cfg.OLT, Interval: cfg.PollInterval, Repo: m.Repo}
	go func() {
		defer func(){ m.mu.Lock(); delete(m.running,cfg.OLT.ID); m.mu.Unlock() }()
		if err := poller.Run(ctx); err != nil && ctx.Err() == nil { log.Printf("OLT runtime stopped id=%s: %v", cfg.OLT.ID, err) }
	}()
	return nil
}

// Stop cancels all active OLT pollers and waits for their cancellation signals.
func (m *RuntimeManager) Stop() {
	m.mu.Lock()
	for id, cancel := range m.running { cancel(); delete(m.running,id) }
	m.mu.Unlock()
}

func (m *RuntimeManager) Running() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.running) }

func (m *RuntimeManager) StartOne(ctx context.Context, cfg ConfiguredOLT) error {
	if cfg.OLT.ID == "" { return fmt.Errorf("OLT id is required") }
	return m.startOne(ctx, cfg)
}
