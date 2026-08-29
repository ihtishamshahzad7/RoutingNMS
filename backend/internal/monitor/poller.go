package monitor

import (
	"context"
	"sync"
	"time"
)

// Probe is the common interface implemented by ICMP, TCP, HTTP and other probes.
type Probe interface {
	Probe(ctx context.Context, device Device) ProbeResult
}

// TCPProbe provides a safe baseline reachability implementation.
type TCPProbe struct {
	Timeout time.Duration
}

func (p TCPProbe) Probe(ctx context.Context, device Device) ProbeResult {
	result := Ping(ctx, device.Address, p.Timeout)
	result.DeviceID = device.ID
	return result
}

// Poller runs bounded concurrent probes. A slow/unreachable device cannot
// consume an unbounded number of goroutines.
type Poller struct {
	Workers int
	Probe   Probe
}

func (p Poller) Run(ctx context.Context, devices []Device) []ProbeResult {
	workers := p.Workers
	if workers < 1 {
		workers = 8
	}
	if workers > len(devices) && len(devices) > 0 {
		workers = len(devices)
	}

	jobs := make(chan Device)
	results := make(chan ProbeResult, len(devices))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for device := range jobs {
				if !device.Enabled {
					continue
				}
				results <- p.Probe.Probe(ctx, device)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, device := range devices {
			select {
			case jobs <- device:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]ProbeResult, 0, len(devices))
	for result := range results {
		out = append(out, result)
	}
	return out
}
