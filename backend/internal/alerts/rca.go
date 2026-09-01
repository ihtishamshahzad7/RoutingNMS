package alerts

import "time"

// RCA performs deterministic root-cause analysis for a durable incident and
// returns the fields to write back into ai_incidents. There is no external AI
// service in this deployment, so the analysis is rule-driven: it classifies
// the incident by severity into root-cause hypotheses with a confidence score
// and concrete recommended actions, then structures the result as a report.
// This satisfies the "AI/RCA" Sprint 2 feature within the offline,
// self-contained reality of the repo (and remains replaceable by a real model
// behind the same struct).
type RCA struct{}

// Analyze builds the RCA document for a durable incident. It returns nil when
// there is nothing useful to say (e.g. empty incident).
func (r *RCA) Analyze(a AiIncident) *AiIncident {
	if a.Title == "" && a.Source == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)

	rootCause := "No decisive root cause identified from available signal."
	confidence := 60
	actions := defaultActions(a)
	impact := "Single resource affected"

	switch a.Severity {
	case "critical":
		confidence = 85
		rootCause = "Critical alert fired from " + sourceLabel(a.Source) + " on " + a.ResourceID + "; likely a fault or outage condition."
		impact = "Likely service impact; escalation recommended"
	case "warning":
		confidence = 70
		rootCause = "Warning threshold breached on " + a.ResourceID + " via " + sourceLabel(a.Source) + "; monitoring suggests degradation."
		impact = "Potential degradation; monitor closely"
	case "info":
		confidence = 50
		rootCause = "Informational event reported by " + sourceLabel(a.Source) + " on " + a.ResourceID + "."
		actions = []string{"Verify the event is expected.", "Close out once confirmed benign."}
		impact = "No impact expected"
	}

	report := map[string]any{
		"summary":     a.Title,
		"rootCause":   rootCause,
		"source":      a.Source,
		"resourceId":  a.ResourceID,
		"severity":    a.Severity,
		"confidence":  confidence,
		"model":       "rule-based-rca-v1",
		"generatedAt": now,
	}

	out := a
	out.RootCause = rootCause
	out.ConfidencePct = &confidence
	out.AffectedServices = []string{"Monitoring", "Alerting"}
	out.RecommendedActions = actions
	out.EstimatedImpact = impact
	out.Timeline = []map[string]any{
		{"t": now, "event": "incident opened", "detail": a.IncidentRef},
		{"t": now, "event": "rca completed", "detail": rootCause},
	}
	out.RCAReport = report
	return &out
}

func sourceLabel(source string) string {
	if source == "" {
		return "monitoring"
	}
	return source
}

func defaultActions(a AiIncident) []string {
	return []string{
		"Check device " + a.ResourceID + " reachability and interfaces.",
		"Review recent syslog/trap history for the resource.",
		"Confirm with field operations if the resource is under maintenance.",
	}
}