package olt

import (
    "context"
    "fmt"
    "time"
)

// Poller periodically refreshes OLT topology through the vendor-neutral
// adapter contract. The adapter owns device-specific discovery and polling.
type Poller struct {
    Adapter Adapter
    OLT OLT
    Interval time.Duration
    OnResult func(PollResult)
}

func (p Poller) Run(ctx context.Context) error {
    if p.Adapter == nil { return fmt.Errorf("OLT adapter is required") }
    interval := p.Interval
    if interval < 30*time.Second { interval = 60*time.Second }
    if err := p.poll(ctx); err != nil && ctx.Err() == nil { return err }
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return ctx.Err()
        case <-ticker.C:
            if err := p.poll(ctx); err != nil && ctx.Err() == nil { return err }
        }
    }
}

func (p Poller) poll(ctx context.Context) error {
    ports, err := p.Adapter.Discover(ctx, p.OLT)
    if err != nil { return fmt.Errorf("discover PONs: %w", err) }
    result := PollResult{PolledAt: time.Now().UTC()}
    for _, port := range ports {
        onus, err := p.Adapter.DiscoverONUs(ctx, p.OLT, port)
        if err != nil { return fmt.Errorf("discover ONUs on %s: %w", port.Name, err) }
        result.PONs = append(result.PONs, port)
        result.ONUs = append(result.ONUs, onus...)
    }
    if p.OnResult != nil { p.OnResult(result) }
    return nil
}
