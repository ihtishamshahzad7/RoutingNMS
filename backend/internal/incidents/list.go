package incidents

import "sort"

func (e *Engine) List(status *Status, severity *Severity) []Incident {
	e.mu.Lock(); defer e.mu.Unlock()
	out := make([]Incident, 0, len(e.items))
	for _, i := range e.items {
		if status != nil && i.Status != *status { continue }
		if severity != nil && i.Severity != *severity { continue }
		out = append(out, i)
	}
	sort.Slice(out, func(a,b int) bool { return out[a].StartedAt.After(out[b].StartedAt) })
	return out
}
