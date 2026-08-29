package olt

import (
    "context"
    "fmt"
    "sync"
    "time"
)

type RuntimeState struct {
    OLTID string `json:"oltId"`
    Name string `json:"name"`
    Running bool `json:"running"`
    StartedAt *time.Time `json:"startedAt,omitempty"`
    LastSuccess *time.Time `json:"lastSuccess,omitempty"`
    LastError string `json:"lastError,omitempty"`
    PollCount uint64 `json:"pollCount"`
}

type RuntimeManager struct {
    config ConfigService
    repo Repository
    mu sync.RWMutex
    states map[string]*RuntimeState
    cancel context.CancelFunc
    wg sync.WaitGroup
}

func NewRuntimeManager(config ConfigService, repo Repository) *RuntimeManager {
    return &RuntimeManager{config: config, repo: repo, states: make(map[string]*RuntimeState)}
}

func (m *RuntimeManager) Start(ctx context.Context) error {
    olts, err := m.config.LoadEnabled(ctx)
    if err != nil { return err }
    if m.cancel != nil { return fmt.Errorf("OLT runtime already started") }
    runCtx, cancel := context.WithCancel(ctx)
    m.cancel = cancel
    for _, c := range olts {
        cfg := c
        now := time.Now().UTC()
        m.mu.Lock()
        m.states[cfg.OLT.ID] = &RuntimeState{OLTID: cfg.OLT.ID, Name: cfg.OLT.Name, Running: true, StartedAt: &now}
        m.mu.Unlock()
        adapter, ok := m.config.Profiles.AdapterFor(cfg.Profile, cfg.SNMP)
        if !ok { m.setError(cfg.OLT.ID, fmt.Errorf("no adapter for profile %s", cfg.Profile.Name)); continue }
        p := Poller{Adapter: adapter, OLT: cfg.OLT, Interval: cfg.PollInterval, Repo: m.repo}
        m.wg.Add(1)
        go func(){ defer m.wg.Done(); err := p.Run(runCtx); if err != nil && runCtx.Err() == nil { m.setError(cfg.OLT.ID, err) }; m.setStopped(cfg.OLT.ID) }()
    }
    return nil
}

func (m *RuntimeManager) Stop() { if m.cancel != nil { m.cancel(); m.wg.Wait(); m.cancel = nil } }
func (m *RuntimeManager) Running() int { m.mu.RLock(); defer m.mu.RUnlock(); n:=0; for _,s:=range m.states { if s.Running { n++ } }; return n }
func (m *RuntimeManager) States() []RuntimeState { m.mu.RLock(); defer m.mu.RUnlock(); out:=make([]RuntimeState,0,len(m.states)); for _,s:=range m.states { out=append(out,*s) }; return out }
func (m *RuntimeManager) setError(id string, err error) { m.mu.Lock(); defer m.mu.Unlock(); if s:=m.states[id]; s!=nil { s.LastError=err.Error(); s.Running=false } }
func (m *RuntimeManager) setStopped(id string) { m.mu.Lock(); defer m.mu.Unlock(); if s:=m.states[id]; s!=nil { s.Running=false } }
