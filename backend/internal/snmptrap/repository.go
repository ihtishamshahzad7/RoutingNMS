package snmptrap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

// Varbind is the JSON-serializable shape stored per trap; kept separate
// from gosnmp's own SnmpPDU type so storage isn't coupled to the library's
// internal representation (and so it round-trips cleanly through JSONB).
type Varbind struct {
	OID   string `json:"oid"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ReceivedTrap struct {
	SourceIP      string
	Version       string // v1|v2c|v3
	TrapOID       string
	EnterpriseOID string
	GenericTrap   *int
	SpecificTrap  *int
	Varbinds      []Varbind
}

type StoredTrap struct {
	ID            int64     `json:"id"`
	ReceivedAt    time.Time `json:"receivedAt"`
	SourceIP      string    `json:"sourceIp"`
	Version       string    `json:"snmpVersion"`
	TrapOID       string    `json:"trapOid"`
	EnterpriseOID string    `json:"enterpriseOid,omitempty"`
	GenericTrap   *int      `json:"genericTrap,omitempty"`
	SpecificTrap  *int      `json:"specificTrap,omitempty"`
	Varbinds      []Varbind `json:"varbinds"`
	MatchedRuleID *int64    `json:"matchedRuleId,omitempty"`
	Severity      string    `json:"severity"`
}

// ListRules returns all trap rules ordered so the most specific OID
// pattern is tried first (longer literal patterns before the catch-all).
func (r Repository) ListRules(ctx context.Context) ([]Rule, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("snmptrap repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,name,oid_pattern,severity,enabled,COALESCE(notify_email,''),COALESCE(notify_webhook_url,'') FROM trap_rules ORDER BY length(oid_pattern) DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.OIDPattern, &rule.Severity, &rule.Enabled, &rule.NotifyEmail, &rule.NotifyWebhookURL); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r Repository) CreateRule(ctx context.Context, rule Rule) (Rule, error) {
	if r.DB == nil {
		return Rule{}, fmt.Errorf("snmptrap repository is not initialized")
	}
	if rule.OIDPattern == "" {
		rule.OIDPattern = "*"
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	err := r.DB.QueryRow(ctx, `INSERT INTO trap_rules (name,oid_pattern,severity,enabled,notify_email,notify_webhook_url) VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')) RETURNING id`,
		rule.Name, rule.OIDPattern, rule.Severity, rule.Enabled, rule.NotifyEmail, rule.NotifyWebhookURL).Scan(&rule.ID)
	return rule, err
}

func (r Repository) DeleteRule(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("snmptrap repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM trap_rules WHERE id=$1`, id)
	return err
}

// Store persists a received trap, matching it against the current rule set
// first so the resulting severity/matched rule is recorded alongside it.
func (r Repository) Store(ctx context.Context, t ReceivedTrap) (StoredTrap, error) {
	if r.DB == nil {
		return StoredTrap{}, fmt.Errorf("snmptrap repository is not initialized")
	}
	rules, err := r.ListRules(ctx)
	if err != nil {
		return StoredTrap{}, fmt.Errorf("load rules: %w", err)
	}
	matchOID := t.TrapOID
	if matchOID == "" {
		matchOID = t.EnterpriseOID
	}
	severity := "info"
	var matchedID *int64
	if rule, ok := FirstMatch(rules, matchOID); ok {
		severity = rule.Severity
		id := rule.ID
		matchedID = &id
	}

	vbJSON, err := json.Marshal(t.Varbinds)
	if err != nil {
		return StoredTrap{}, err
	}

	var st StoredTrap
	err = r.DB.QueryRow(ctx, `INSERT INTO snmp_traps (source_ip,snmp_version,trap_oid,enterprise_oid,generic_trap,specific_trap,varbinds,matched_rule_id,severity)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9) RETURNING id,received_at`,
		t.SourceIP, t.Version, t.TrapOID, t.EnterpriseOID, t.GenericTrap, t.SpecificTrap, vbJSON, matchedID, severity).Scan(&st.ID, &st.ReceivedAt)
	if err != nil {
		return StoredTrap{}, err
	}
	st.SourceIP = t.SourceIP
	st.Version = t.Version
	st.TrapOID = t.TrapOID
	st.EnterpriseOID = t.EnterpriseOID
	st.GenericTrap = t.GenericTrap
	st.SpecificTrap = t.SpecificTrap
	st.Varbinds = t.Varbinds
	st.MatchedRuleID = matchedID
	st.Severity = severity
	return st, nil
}

func (r Repository) List(ctx context.Context, limit int, sourceIP string) ([]StoredTrap, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("snmptrap repository is not initialized")
	}
	sql := `SELECT id,received_at,source_ip,snmp_version,trap_oid,COALESCE(enterprise_oid,''),generic_trap,specific_trap,varbinds,matched_rule_id,severity FROM snmp_traps WHERE 1=1`
	args := []any{}
	if sourceIP != "" {
		args = append(args, sourceIP)
		sql += fmt.Sprintf(` AND source_ip = $%d`, len(args))
	}
	args = append(args, limit)
	sql += fmt.Sprintf(` ORDER BY received_at DESC LIMIT $%d`, len(args))

	rows, err := r.DB.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StoredTrap{}
	for rows.Next() {
		var st StoredTrap
		var vbRaw []byte
		if err := rows.Scan(&st.ID, &st.ReceivedAt, &st.SourceIP, &st.Version, &st.TrapOID, &st.EnterpriseOID, &st.GenericTrap, &st.SpecificTrap, &vbRaw, &st.MatchedRuleID, &st.Severity); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(vbRaw, &st.Varbinds)
		out = append(out, st)
	}
	return out, rows.Err()
}

// PruneOlderThan deletes stored traps older than age, mirroring the syslog
// package's retention approach so trap history doesn't grow unbounded.
func (r Repository) PruneOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("snmptrap repository is not initialized")
	}
	tag, err := r.DB.Exec(ctx, `DELETE FROM snmp_traps WHERE received_at < $1`, time.Now().Add(-age))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
