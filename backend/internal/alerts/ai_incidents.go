package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/incidents"
)

// AiIncident is a durable `ai_incidents` row (migration 0015): one per opened
// incident, carrying the RCA fields the analyzer writes back. This is the
// Incident Hub's source of truth for history + root-cause analysis, while
// internal/incidents remains the live in-memory store backing /api/incidents
// and the SSE stream.
type AiIncident struct {
	ID          int64     `json:"id"`
	IncidentRef string    `json:"incidentRef"`
	Status      string    `json:"status"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Source      string    `json:"source"`
	ResourceID  string    `json:"resourceId"`
	DeviceID    *int64    `json:"deviceId,omitempty"`
	TriggeredAt time.Time `json:"triggeredAt"`

	RootCause          string           `json:"rootCause,omitempty"`
	ConfidencePct      *int             `json:"confidencePct,omitempty"`
	AffectedServices   []string         `json:"affectedServices,omitempty"`
	RecommendedActions []string         `json:"recommendedActions,omitempty"`
	EstimatedImpact    string           `json:"estimatedImpact,omitempty"`
	Timeline           []map[string]any `json:"timeline,omitempty"`
	RCAReport          map[string]any   `json:"rcaReport,omitempty"`
	RCACompletedAt     *time.Time       `json:"rcaCompletedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SaveAiIncident inserts a new durable incident and returns it with its id.
func (r Repository) SaveAiIncident(ctx context.Context, a AiIncident) (AiIncident, error) {
	if r.DB == nil {
		return AiIncident{}, fmt.Errorf("alerts repository is not initialized")
	}
	var deviceID any
	if a.DeviceID != nil {
		deviceID = *a.DeviceID
	}
	if a.Severity == "" {
		a.Severity = "warning"
	}
	if a.Status == "" {
		a.Status = "open"
	}
	err := r.DB.QueryRow(ctx, `INSERT INTO ai_incidents
		(incident_ref,status,severity,title,source,resource_id,device_id,triggered_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at,updated_at`,
		a.IncidentRef, a.Status, a.Severity, a.Title, a.Source, a.ResourceID,
		deviceID, a.TriggeredAt).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return AiIncident{}, err
	}
	return sanitizeNullIncident(a), nil
}

// UpdateAiIncidentRCA writes the analyzer's RCA output back to a resolved
// incident and marks it rca_completed_at. id is the ai_incidents primary key.
func (r Repository) UpdateAiIncidentRCA(ctx context.Context, id int64, a AiIncident) error {
	if r.DB == nil {
		return fmt.Errorf("alerts repository is not initialized")
	}
	now := time.Now().UTC()
	svcs, err := json.Marshal(a.AffectedServices)
	if err != nil {
		return err
	}
	actions, err := json.Marshal(a.RecommendedActions)
	if err != nil {
		return err
	}
	timeline, err := json.Marshal(a.Timeline)
	if err != nil {
		return err
	}
	report, err := json.Marshal(a.RCAReport)
	if err != nil {
		return err
	}
	_, err = r.DB.Exec(ctx, `UPDATE ai_incidents SET
		root_cause=$2, confidence_pct=$3, affected_services=$4, recommended_actions=$5,
		estimated_impact=$6, timeline=$7, rca_report=$8, rca_completed_at=$9,
		status=$10, updated_at=NOW()
		WHERE id=$1`,
		id, a.RootCause, a.ConfidencePct, svcs, actions, a.EstimatedImpact,
		timeline, report, now, a.Status)
	return err
}

// ListAiIncidents returns durable incidents, newest-first, with optional
// status/severity filters, for the Incident Hub.
func (r Repository) ListAiIncidents(ctx context.Context, status, severity string, limit int) ([]AiIncident, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("alerts repository is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	sql := `SELECT id,incident_ref,status,severity,title,COALESCE(source,''),COALESCE(resource_id,''),device_id,
		triggered_at,COALESCE(root_cause,''),confidence_pct,affected_services,recommended_actions,
		COALESCE(estimated_impact,''),timeline,rca_report,rca_completed_at,created_at,updated_at
		FROM ai_incidents WHERE 1=1`
	args := []any{}
	if status != "" {
		args = append(args, status)
		sql += fmt.Sprintf(` AND status=$%d`, len(args))
	}
	if severity != "" {
		args = append(args, severity)
		sql += fmt.Sprintf(` AND severity=$%d`, len(args))
	}
	args = append(args, limit)
	sql += fmt.Sprintf(` ORDER BY triggered_at DESC LIMIT $%d`, len(args))

	rows, err := r.DB.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AiIncident{}
	for rows.Next() {
		a, err := scanAiIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAiIncident returns one durable incident by id.
func (r Repository) GetAiIncident(ctx context.Context, id int64) (AiIncident, error) {
	if r.DB == nil {
		return AiIncident{}, fmt.Errorf("alerts repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,incident_ref,status,severity,title,COALESCE(source,''),COALESCE(resource_id,''),device_id,
		triggered_at,COALESCE(root_cause,''),confidence_pct,affected_services,recommended_actions,
		COALESCE(estimated_impact,''),timeline,rca_report,rca_completed_at,created_at,updated_at
		FROM ai_incidents WHERE id=$1`, id)
	if err != nil {
		return AiIncident{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return AiIncident{}, sql.ErrNoRows
	}
	return scanAiIncident(rows)
}

// SetAiIncidentStatus transitions a durable incident's status.
func (r Repository) SetAiIncidentStatus(ctx context.Context, id int64, status string) error {
	if r.DB == nil {
		return fmt.Errorf("alerts repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE ai_incidents SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

// CloseAiIncidentByRef marks every still-open durable incident carrying the
// given incident_ref (the same "source:resource:code" id the live
// incidents.Engine keys on -- see Correlator.Process) as resolved. Used by
// Evaluator.resolve (Feature 1.4) to close out the incident record a fired
// alert opened once its underlying metric recovers, rather than leaving it
// "open" forever with only a resolved notification sent. A no-op (0 rows
// affected, no error) when nothing matching is open, which is the common
// case for an alert that resolved before this fix existed.
func (r Repository) CloseAiIncidentByRef(ctx context.Context, incidentRef string) error {
	if r.DB == nil {
		return fmt.Errorf("alerts repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE ai_incidents SET status='resolved', updated_at=NOW()
		WHERE incident_ref=$1 AND status <> 'resolved'`, incidentRef)
	return err
}

type aiIncidentScanner interface {
	Scan(dest ...any) error
}

func scanAiIncident(rows aiIncidentScanner) (AiIncident, error) {
	var a AiIncident
	var deviceID *int64
	var confidence *int
	var affected, actions, timeline, report []byte
	var rcaAt *time.Time
	err := rows.Scan(&a.ID, &a.IncidentRef, &a.Status, &a.Severity, &a.Title, &a.Source,
		&a.ResourceID, &deviceID, &a.TriggeredAt, &a.RootCause, &confidence,
		&affected, &actions, &a.EstimatedImpact, &timeline, &report, &rcaAt,
		&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return AiIncident{}, err
	}
	a.DeviceID = deviceID
	a.ConfidencePct = confidence
	a.RCACompletedAt = rcaAt
	_ = json.Unmarshal(affected, &a.AffectedServices)
	_ = json.Unmarshal(actions, &a.RecommendedActions)
	_ = json.Unmarshal(timeline, &a.Timeline)
	_ = json.Unmarshal(report, &a.RCAReport)
	return a, nil
}

func sanitizeNullIncident(a AiIncident) AiIncident {
	if a.ID == 0 {
		return a
	}
	if a.IncidentRef == "" {
		a.IncidentRef = strconv.FormatInt(a.ID, 10)
	}
	return a
}

// IncidentBridge wires the alert engine's fired alerts into the live incident
// system (internal/incidents Engine + SSE Stream) AND the durable ai_incidents
// table (Incident Hub + RCA). It is the connection point that previously did
// not exist: nothing in the codebase ever fed the in-memory incident engine.
type IncidentBridge struct {
	Live     *incidents.Engine
	Stream   *incidents.Stream
	Repo     Repository
	Analyzer *RCA
}

// Open converts a fired alert into an incident, persists it, runs RCA, and
// pushes it to the live SSE stream. Returns the durable row.
func (b IncidentBridge) Open(ctx context.Context, a Alert) (AiIncident, error) {
	liveIncident, err := incidents.Correlator{Engine: b.Live}.Process(incidents.Alert{
		Code:       a.RuleKey,
		Severity:   incidents.Severity(a.Severity),
		Title:      incidentTitle(a),
		Source:     "alert-rule",
		ResourceID: a.DeviceID,
	})
	if err != nil {
		return AiIncident{}, err
	}

	var deviceID *int64
	if id, perr := strconv.ParseInt(a.DeviceID, 10, 64); perr == nil && id > 0 {
		deviceID = &id
	}

	durable, err := b.Repo.SaveAiIncident(ctx, AiIncident{
		IncidentRef: liveIncident.ID,
		Status:      string(liveIncident.Status),
		Severity:    string(liveIncident.Severity),
		Title:       liveIncident.Title,
		Source:      "alert-rule",
		ResourceID:  liveIncident.ResourceID,
		DeviceID:    deviceID,
		TriggeredAt: liveIncident.StartedAt,
	})
	if err != nil {
		return AiIncident{}, err
	}

	// Run RCA and persist it (best-effort: a failed analysis must not drop
	// the incident itself).
	if b.Analyzer != nil {
		if report := b.Analyzer.Analyze(durable); report != nil {
			_ = b.Repo.UpdateAiIncidentRCA(ctx, durable.ID, *report)
		}
	}

	if b.Stream != nil {
		b.Stream.Publish(liveIncident)
	}
	return durable, nil
}

func incidentTitle(a Alert) string {
	return "Alert " + a.RuleKey + " breached on " + a.DeviceID
}

// incidentRef reconstructs the same deterministic id Correlator.Process
// derives when Open first creates the incident ("source:resource:code",
// here always "alert-rule:<deviceID>:<ruleKey>" since Open never sets
// ParentResourceID) -- letting Close resolve the right live+durable
// incident from just a rule key and device id, with no extra state to
// track across the breach's lifetime.
func incidentRef(ruleKey, deviceID string) string {
	return "alert-rule:" + deviceID + ":" + ruleKey
}

// Close resolves the live in-memory incident (publishing the transition to
// the SSE stream so the Incident Hub updates immediately) and marks the
// matching durable ai_incidents row(s) resolved. Called by Evaluator.resolve
// when a previously-breaching rule's condition clears -- the counterpart to
// Open, so a recovered alert's incident record doesn't stay "open" forever
// with only a notification to show for the recovery.
func (b IncidentBridge) Close(ctx context.Context, ruleKey, deviceID string) error {
	ref := incidentRef(ruleKey, deviceID)
	if b.Live != nil {
		if inc, err := b.Live.Resolve(ref); err == nil && b.Stream != nil {
			b.Stream.Publish(inc)
		}
		// A resolve error (e.g. "incident not found", already resolved by an
		// earlier tick, or the live engine was restarted since Open) is not
		// fatal -- the durable-row update below is still attempted and is
		// what actually matters for the Incident Hub's history.
	}
	return b.Repo.CloseAiIncidentByRef(ctx, ref)
}
