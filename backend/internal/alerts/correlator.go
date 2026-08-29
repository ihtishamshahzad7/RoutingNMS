package alerts

import "time"

type Incident struct {
	ID          string
	Title       string
	Severity    Severity
	DeviceIDs   []string
	AlertKeys   []string
	StartedAt   time.Time
}

// Correlate groups alerts sharing a root device. This is the first layer of
// event correlation; topology-aware dependency correlation will extend it.
func Correlate(alerts []Alert, rootDeviceID string) *Incident {
	if len(alerts) == 0 { return nil }
	incident := &Incident{ID: "incident-" + rootDeviceID, Title: "Network incident", DeviceIDs: []string{rootDeviceID}, StartedAt: alerts[0].StartedAt}
	for _, alert := range alerts {
		incident.AlertKeys = append(incident.AlertKeys, alert.Key)
		if alert.Severity == Critical { incident.Severity = Critical } else if incident.Severity == "" && alert.Severity == Warning { incident.Severity = Warning }
	}
	if incident.Severity == "" { incident.Severity = Info }
	return incident
}
