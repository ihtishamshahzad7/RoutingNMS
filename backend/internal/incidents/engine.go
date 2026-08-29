package incidents

import (
	"fmt"
	"sync"
	"time"
)

type Status string
const (
	Open Status = "open"
	Acknowledged Status = "acknowledged"
	Resolved Status = "resolved"
)

type Severity string
const (
	Critical Severity = "critical"
	Warning Severity = "warning"
	Info Severity = "info"
)

type Incident struct {
	ID string `json:"id"`
	Status Status `json:"status"`
	Severity Severity `json:"severity"`
	Title string `json:"title"`
	Source string `json:"source"`
	ResourceID string `json:"resourceId"`
	StartedAt time.Time `json:"startedAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

type Engine struct { mu sync.Mutex; items map[string]Incident }
func NewEngine() *Engine { return &Engine{items: map[string]Incident{}} }

func (e *Engine) Open(id string, severity Severity, title, source, resourceID string) (Incident, error) {
	if id == "" || title == "" || resourceID == "" { return Incident{}, fmt.Errorf("incident ID, title and resource ID are required") }
	e.mu.Lock(); defer e.mu.Unlock()
	if existing, ok := e.items[id]; ok && existing.Status != Resolved { return existing, nil }
	i := Incident{ID:id, Status:Open, Severity:severity, Title:title, Source:source, ResourceID:resourceID, StartedAt:time.Now().UTC()}
	e.items[id] = i
	return i, nil
}

func (e *Engine) Acknowledge(id string) (Incident, error) {
	e.mu.Lock(); defer e.mu.Unlock(); i, ok := e.items[id]; if !ok { return Incident{}, fmt.Errorf("incident not found") }; if i.Status == Resolved { return i, fmt.Errorf("incident already resolved") }; now:=time.Now().UTC(); i.Status=Acknowledged; i.AcknowledgedAt=&now; e.items[id]=i; return i,nil
}

func (e *Engine) Resolve(id string) (Incident, error) {
	e.mu.Lock(); defer e.mu.Unlock(); i, ok := e.items[id]; if !ok { return Incident{}, fmt.Errorf("incident not found") }; now:=time.Now().UTC(); i.Status=Resolved; i.ResolvedAt=&now; e.items[id]=i; return i,nil
}

func (e *Engine) Get(id string) (Incident, bool) { e.mu.Lock(); defer e.mu.Unlock(); i,ok:=e.items[id]; return i,ok }
