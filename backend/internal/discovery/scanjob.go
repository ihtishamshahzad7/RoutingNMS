package discovery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Job tracks one in-progress or completed subnet scan. Kept in memory only
// (like incidents.Engine and olt.RuntimeManager's state maps elsewhere in
// this codebase) -- a scan is an operator-triggered, short-lived UI
// interaction (scan -> review -> import), not something that needs to
// survive a process restart.
type Job struct {
	ID          string    `json:"id"`
	CIDR        string    `json:"cidr"`
	Status      string    `json:"status"` // running|done|error
	Total       int       `json:"total"`
	Scanned     int       `json:"scanned"`
	Results     []Found   `json:"results"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt,omitempty"`
	credentials snmp.Credentials
	port        uint16
}

// Manager runs and tracks subnet scan jobs.
type Manager struct {
	Collector snmp.Collector
	mu        sync.Mutex
	jobs      map[string]*Job
}

func NewManager() *Manager { return &Manager{jobs: make(map[string]*Job)} }

// Start launches a scan in the background and returns its job id
// immediately; poll Get(id) for progress and results.
func (m *Manager) Start(cidr string, creds snmp.Credentials, port uint16, timeoutMS, concurrency int) (*Job, error) {
	hosts, err := ExpandCIDR(cidr)
	if err != nil {
		return nil, err
	}
	if concurrency <= 0 || concurrency > 128 {
		concurrency = 64
	}

	job := &Job{ID: newJobID(), CIDR: cidr, Status: "running", Total: len(hosts), StartedAt: time.Now().UTC(), credentials: creds, port: port}
	m.mu.Lock()
	if m.jobs == nil {
		m.jobs = make(map[string]*Job)
	}
	m.jobs[job.ID] = job
	m.mu.Unlock()

	go m.run(job, hosts, timeoutMS, concurrency)
	return job, nil
}

func (m *Manager) run(job *Job, hosts []string, timeoutMS, concurrency int) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, host := range hosts {
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			found, ok := ProbeOne(ctx, m.Collector, addr, job.port, job.credentials, timeoutMS)
			mu.Lock()
			job.Scanned++
			if ok {
				job.Results = append(job.Results, found)
			}
			mu.Unlock()
		}(host)
	}
	wg.Wait()

	m.mu.Lock()
	job.Status = "done"
	job.FinishedAt = time.Now().UTC()
	m.mu.Unlock()
}

// Get returns a snapshot-safe copy of the job's current state (results
// slice included) for the status API, or false if the id is unknown.
func (m *Manager) Get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	snapshot := *job
	snapshot.Results = append([]Found(nil), job.Results...)
	return snapshot, true
}

// Credentials returns the SNMP credentials + port a job was scanned with,
// so the import step can reuse them without the caller re-sending secrets.
func (m *Manager) Credentials(id string) (snmp.Credentials, uint16, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return snmp.Credentials{}, 0, false
	}
	return job.credentials, job.port, true
}

func newJobID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "scan-" + hex.EncodeToString(b)
}
