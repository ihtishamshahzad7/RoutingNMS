package olt

import (
    "context"
    "fmt"
    "time"
)

// Poller periodically refreshes OLT topology through a vendor adapter.
// Persistence and alert emission are deliberately injected so the OLT layer
// stays independent of PostgreSQL and the notification transport.
type Poller struct {
    Adapter Adapter
    Interval time.Duration
    OnResult func(PollResult)
}

func (p Poller) Run(ctx context.Context) error {
    if p.Adapter == nil { return fmt.Errorf("OLT adapter is required") }
    interval := p.Interval
    if interval < 30*time.Second { interval = 60*time.Second }
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        if err := p.poll(ctx); err != nil && ctx.Err() == nil { return err }
        select {
        case <-ctx.Done(): return ctx.Err()
        case <-ticker.C:
        }
    }
}

func (p Poller) poll(_ context.Context) error {
    pons, err := p.Adapter.DiscoverPONs()
    if err != nil { return fmt.Errorf("discover PONs: %w", err) }
    result := PollResult{PONs: pons, PolledAt: time.Now().UTC()}
    for _, pon := range pons {
        onus, err := p.Adapter.DiscoverONUs(pon)
        if err != nil { return fmt.Errorf("discover ONUs on %s: %w", pon.Name, err) }
        result.ONUs = append(result.ONUs, onus...)
    }
    if p.OnResult != nil { p.OnResult(result) }
    return nil
}
