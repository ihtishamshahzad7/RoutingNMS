package incidents

import (
	"fmt"
	"strings"
)

// Alert is the normalized input accepted from SNMP, OLT, ICMP and vendor adapters.
type Alert struct {
	Code string `json:"code"`
	Severity Severity `json:"severity"`
	Title string `json:"title"`
	Source string `json:"source"`
	ResourceID string `json:"resourceId"`
	ParentResourceID string `json:"parentResourceId,omitempty"`
}

// Correlator turns related child alerts into a single root incident when a
// parent resource is supplied. This prevents an outage from flooding the NOC.
type Correlator struct { Engine *Engine }

func (c Correlator) Process(a Alert) (Incident, error) {
	if c.Engine == nil { return Incident{}, fmt.Errorf("incident engine is required") }
	resource := a.ResourceID
	if strings.TrimSpace(a.ParentResourceID) != "" { resource = a.ParentResourceID }
	id := a.Source + ":" + resource + ":" + a.Code
	title := a.Title
	if title == "" { title = a.Code }
	return c.Engine.Open(id, a.Severity, title, a.Source, resource)
}
