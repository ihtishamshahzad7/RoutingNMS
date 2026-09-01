package mikrotik

import (
    "context"
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metrics"
    "github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Standard MikroTik RouterOS OIDs. RouterOS exposes these through standard
// SNMP objects, so the adapter remains usable without the RouterOS API.
const (
    OIDCPU     = ".1.3.6.1.4.1.14988.1.1.3.10.0"
    OIDMemory  = ".1.3.6.1.4.1.14988.1.1.3.1.0"
    OIDFreeMem = ".1.3.6.1.4.1.14988.1.1.3.2.0"
    OIDTemp    = ".1.3.6.1.4.1.14988.1.1.3.9.0"
)

type Adapter struct { Collector snmp.Collector; Writer metrics.Writer }

type Result struct {
    CPUPercent float64 `json:"cpuPercent"`
    MemoryTotalBytes uint64 `json:"memoryTotalBytes"`
    MemoryFreeBytes uint64 `json:"memoryFreeBytes"`
    TemperatureC float64 `json:"temperatureC"`
}

func (a Adapter) Poll(ctx context.Context, target snmp.Target) (Result, error) {
    client, err := a.Collector.Connect(ctx, target)
    if err != nil { return Result{}, err }
    defer client.Conn.Close()
    response, err := client.Get([]string{OIDCPU, OIDMemory, OIDFreeMem, OIDTemp})
    if err != nil { return Result{}, fmt.Errorf("mikrotik system poll: %w", err) }
    result := Result{}
    for _, pdu := range response.Variables {
        value := number(pdu.Value)
        switch pdu.Name { case OIDCPU: result.CPUPercent=value; case OIDMemory: result.MemoryTotalBytes=uint64(value); case OIDFreeMem: result.MemoryFreeBytes=uint64(value); case OIDTemp: result.TemperatureC=value }
    }
    return result, nil
}

func (a Adapter) Samples(ctx context.Context, target snmp.Target, deviceID string) error {
    r, err := a.Poll(ctx,target); if err != nil { return err }
    now := time.Now().UTC(); labels := map[string]string{"device_id":deviceID,"vendor":"mikrotik"}
    used := float64(0); if r.MemoryTotalBytes > 0 { used = float64(r.MemoryTotalBytes-r.MemoryFreeBytes)/float64(r.MemoryTotalBytes)*100 }
    return a.Writer.Write(ctx, []metrics.Sample{
        {Name:"mikrotik_cpu_percent",Value:r.CPUPercent,Timestamp:now,Labels:labels},
        {Name:"mikrotik_memory_used_percent",Value:used,Timestamp:now,Labels:labels},
        {Name:"mikrotik_memory_total_bytes",Value:float64(r.MemoryTotalBytes),Timestamp:now,Labels:labels},
        {Name:"mikrotik_memory_free_bytes",Value:float64(r.MemoryFreeBytes),Timestamp:now,Labels:labels},
        {Name:"mikrotik_temperature_celsius",Value:r.TemperatureC,Timestamp:now,Labels:labels},
    })
}

func number(v any) float64 { switch x:=v.(type) { case uint64:return float64(x); case uint32:return float64(x); case int:return float64(x); case int64:return float64(x); case float64:return x; case string:n,_:=strconv.ParseFloat(strings.TrimSpace(x),64);return n; case []byte:n,_:=strconv.ParseFloat(strings.TrimSpace(string(x)),64);return n; default:return 0 } }
