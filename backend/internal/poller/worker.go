package poller

import (
    "context"
    "log"
    "sync"
    "time"

    "github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metrics"
    "github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type Device struct {
    ID, Name, Address string
    Port uint16
    Credentials snmp.Credentials
    Interval time.Duration
}

type Worker struct {
    Collector snmp.Collector
    Writer metrics.Writer
}

func (w Worker) Run(ctx context.Context, device Device) {
    interval := device.Interval
    if interval < 5*time.Second { interval = 60*time.Second }
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    w.poll(ctx, device)
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C: w.poll(ctx, device)
        }
    }
}

func (w Worker) poll(ctx context.Context, device Device) {
    target := snmp.Target{ID: device.ID, Address: device.Address, Port: device.Port, Credentials: device.Credentials, Timeout: 5*time.Second, Retries: 1}
    result, err := w.Collector.Discover(ctx, target)
    if err != nil { log.Printf("poll device=%s error=%v", device.ID, err); return }
    now := time.Now().UTC()
    samples := []metrics.Sample{
        {Name:"network_device_up", Value:1, Timestamp:now, Labels:map[string]string{"device_id":device.ID}},
        {Name:"network_device_uptime_seconds", Value:float64(result.Uptime)/100.0, Timestamp:now, Labels:map[string]string{"device_id":device.ID}},
        {Name:"network_device_interface_count", Value:float64(len(result.Interfaces)), Timestamp:now, Labels:map[string]string{"device_id":device.ID}},
    }
    for _, iface := range result.Interfaces {
        labels := map[string]string{"device_id":device.ID, "interface":iface.Index}
        samples = append(samples,
            metrics.Sample{Name:"network_interface_admin_up", Value:boolValue(iface.AdminUp), Timestamp:now, Labels:labels},
            metrics.Sample{Name:"network_interface_oper_up", Value:boolValue(iface.OperUp), Timestamp:now, Labels:labels},
            metrics.Sample{Name:"network_interface_in_octets", Value:float64(iface.InOctets), Timestamp:now, Labels:labels},
            metrics.Sample{Name:"network_interface_out_octets", Value:float64(iface.OutOctets), Timestamp:now, Labels:labels},
            metrics.Sample{Name:"network_interface_in_errors", Value:float64(iface.InErrors), Timestamp:now, Labels:labels},
            metrics.Sample{Name:"network_interface_out_errors", Value:float64(iface.OutErrors), Timestamp:now, Labels:labels},
        )
    }
    if err := w.Writer.Write(ctx, samples); err != nil { log.Printf("metrics device=%s error=%v", device.ID, err) }
}

func boolValue(v bool) float64 { if v { return 1 }; return 0 }

type Manager struct { mu sync.Mutex; cancel map[string]context.CancelFunc }
func NewManager() *Manager { return &Manager{cancel:map[string]context.CancelFunc{}} }
func (m *Manager) Start(parent context.Context, w Worker, d Device) { m.mu.Lock(); if old,ok:=m.cancel[d.ID]; ok { old() }; ctx,cancel:=context.WithCancel(parent); m.cancel[d.ID]=cancel; m.mu.Unlock(); go w.Run(ctx,d) }
func (m *Manager) Stop(id string) { m.mu.Lock(); if cancel,ok:=m.cancel[id]; ok { cancel(); delete(m.cancel,id) }; m.mu.Unlock() }
