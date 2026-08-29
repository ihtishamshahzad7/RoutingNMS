package events

import (
	"sync"
	"time"
)

type Type string
const (
	AlertOpened Type = "alert.opened"
	AlertResolved Type = "alert.resolved"
	DeviceStatus Type = "device.status"
	IncidentOpened Type = "incident.opened"
)

type Event struct {
	ID string `json:"id"`
	Type Type `json:"type"`
	OrganizationID string `json:"organizationId"`
	DeviceID string `json:"deviceId,omitempty"`
	Severity string `json:"severity,omitempty"`
	Title string `json:"title"`
	Data map[string]any `json:"data,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Subscriber chan Event

type Bus struct { mu sync.RWMutex; subscribers map[Subscriber]struct{} }
func NewBus() *Bus { return &Bus{subscribers: make(map[Subscriber]struct{})} }
func (b *Bus) Subscribe(buffer int) Subscriber { ch := make(Subscriber, buffer); b.mu.Lock(); b.subscribers[ch] = struct{}{}; b.mu.Unlock(); return ch }
func (b *Bus) Unsubscribe(ch Subscriber) { b.mu.Lock(); if _, ok := b.subscribers[ch]; ok { delete(b.subscribers, ch); close(ch) }; b.mu.Unlock() }
func (b *Bus) Publish(event Event) { if event.CreatedAt.IsZero() { event.CreatedAt = time.Now().UTC() }; b.mu.RLock(); defer b.mu.RUnlock(); for ch := range b.subscribers { select { case ch <- event: default: } } }
