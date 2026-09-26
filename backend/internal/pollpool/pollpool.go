// Package pollpool implements Feature 1.1 of the RoutingNMS build blueprint
// ("Poller Worker Pool").
//
// Scope decision (see claude/roadmap.md Feature 1.1 for the full writeup):
// the original blueprint assumed a NATS JetStream-backed job queue, so that
// polling work could be distributed across multiple worker processes. This
// codebase runs as a single backend process on a single box (confirmed via
// AGENTS.md and this project's own docker-compose: NATS is deployed but has
// no client anywhere in the Go code), so adopting a message-broker job queue
// now would add real operational surface (a new dependency to run, connect
// to, and keep healthy) for a scaling problem this deployment doesn't have
// yet -- there is exactly one process to distribute work across.
//
// What this codebase DOES have is a real, present-day version of the
// problem the blueprint feature is actually about: every existing periodic
// poller (backend/internal/ping, dnscheck, sshcheck, telnetcheck,
// topolinks, devices' sampler, olt, and the generic backend/internal/poller
// package) loops over its device set *sequentially* on each tick, one probe
// at a time, each with its own timeout. A handful of slow or unreachable
// devices in one of those lists therefore linearly stretches out how long a
// full poll cycle takes for every other device behind them in the same
// list -- the exact "adding more monitors shouldn't slow down check
// intervals" complaint the blueprint feature exists to fix.
//
// This package is a small, dependency-free, in-process bounded worker pool:
// callers hand it a slice of items and a per-item work function, and it runs
// them concurrently across a fixed number of workers, returning once every
// item has completed (or the context is cancelled). It's the generic
// primitive every existing sequential poller can drop in without changing
// its own scheduling, storage, or gating logic -- just the "for _, d :=
// range devices { probe(d) }" line changes shape.
//
// If a future feature genuinely needs to distribute polling across more
// than one process (multi-node horizontal scale-out), the natural next step
// is wiring NATS JetStream in front of this same per-item work function --
// this package doesn't preclude that, it just doesn't build it before it's
// needed (flagged as 1.1b below).
package pollpool

import (
	"context"
	"sync"
)

// DefaultWorkers is used when Run is called with workers <= 0. It matches
// the bounded-concurrency default already used by
// backend/internal/monitor/poller.go elsewhere in this codebase, so the two
// don't establish conflicting conventions for "how many things poll at
// once" on the same box.
const DefaultWorkers = 8

// Run executes fn(ctx, item) for every item in items, across `workers`
// concurrent goroutines (clamped to at least 1 and at most len(items)).
// It blocks until every item has been processed or ctx is cancelled, and
// never returns an error itself -- fn is responsible for handling and
// logging its own per-item failures (matching every existing poller's
// current behavior of logging-and-continuing on a single device's probe
// error rather than aborting the whole cycle).
//
// A cancelled ctx stops dispatching new items but does not interrupt an
// in-flight fn call; callers that need per-item timeouts should apply them
// inside fn (every existing poller already does this with
// context.WithTimeout per probe).
func Run[T any](ctx context.Context, items []T, workers int, fn func(ctx context.Context, item T)) {
	if len(items) == 0 {
		return
	}
	if workers <= 0 {
		workers = DefaultWorkers
	}
	if workers > len(items) {
		workers = len(items)
	}

	jobs := make(chan T)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for item := range jobs {
				fn(ctx, item)
			}
		}()
	}

	for _, item := range items {
		select {
		case jobs <- item:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}
