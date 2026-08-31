package olt

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type RuntimeState struct {
	OLTID      string     `json:"oltId"`
	Running    bool       `json:"running"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	LastPollAt *time.Time `json:"lastPollAt,omitempty"`
	LastError  string     `json:"lastError,omitempty"`
	PollCount  uint64     `json:"pollCount"`
}

// RuntimeManager owns the lifecycle of one polling goroutine per enabled OLT.
type RuntimeManager struct {
	Config  ConfigService
	Repo    Repository
	Metrics MetricSampler // zero value (nil DB) is a safe no-op
	mu      sync.Mutex
	running map[string]context.CancelFunc
	state   map[string]RuntimeState
}

func NewRuntimeManager(config ConfigService, repo Repository) *RuntimeManager {
	return &RuntimeManager{Config: config, Repo: repo, running: make(map[string]context.CancelFunc), state: make(map[string]RuntimeState)}
}

func (m *RuntimeManager) Start(ctx context.Context) error {
	configs, err := m.Config.LoadEnabled(ctx)
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if err := m.startOne(ctx, cfg); err != nil {
			log.Printf("OLT runtime start failed id=%s: %v", cfg.OLT.ID, err)
		}
	}
	return nil
}

func (m *RuntimeManager) startOne(parent context.Context, cfg ConfiguredOLT) error {
	adapter, err := m.Config.AdapterFor(cfg)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if _, exists := m.running[cfg.OLT.ID]; exists {
		m.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	now := time.Now().UTC()
	m.running[cfg.OLT.ID] = cancel
	m.state[cfg.OLT.ID] = RuntimeState{OLTID: cfg.OLT.ID, Running: true, StartedAt: &now}
	m.mu.Unlock()
	poller := Poller{Adapter: adapter, OLT: cfg.OLT, Interval: cfg.PollInterval, Repo: m.Repo, OnResult: func(result PollResult) {
		m.recordSuccess(cfg.OLT.ID, result)
		if m.Metrics.Repo.DB != nil {
			metricsCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := m.Metrics.Record(metricsCtx, cfg.OLT.ID, result); err != nil {
				log.Printf("OLT metric history write failed id=%s: %v", cfg.OLT.ID, err)
			}
			cancel()
		}
	}}
	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.running, cfg.OLT.ID)
			s := m.state[cfg.OLT.ID]
			s.Running = false
			m.state[cfg.OLT.ID] = s
			m.mu.Unlock()
		}()
		if err := poller.Run(ctx); err != nil && ctx.Err() == nil {
			m.recordError(cfg.OLT.ID, err)
			log.Printf("OLT runtime stopped id=%s: %v", cfg.OLT.ID, err)
		}
	}()
	return nil
}

func (m *RuntimeManager) recordSuccess(id string, result PollResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state[id]
	now := result.PolledAt
	s.LastPollAt = &now
	s.PollCount++
	s.LastError = ""
	m.state[id] = s
}
func (m *RuntimeManager) recordError(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state[id]
	s.LastError = err.Error()
	m.state[id] = s
}

func (m *RuntimeManager) Stop() {
	m.mu.Lock()
	for id, cancel := range m.running {
		cancel()
		s := m.state[id]
		s.Running = false
		m.state[id] = s
	}
	m.mu.Unlock()
}
func (m *RuntimeManager) Running() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.running) }
func (m *RuntimeManager) StartOne(ctx context.Context, cfg ConfiguredOLT) error {
	if cfg.OLT.ID == "" {
		return fmt.Errorf("OLT id is required")
	}
	return m.startOne(ctx, cfg)
}
func (m *RuntimeManager) States() []RuntimeState {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RuntimeState, 0, len(m.state))
	for _, s := range m.state {
		out = append(out, s)
	}
	return out
}
