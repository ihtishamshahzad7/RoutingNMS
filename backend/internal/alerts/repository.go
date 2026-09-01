package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists named alert rules (`alert_rules`, migration 0016) and
// notification channels (`notification_channels`, migration 0017). It is the
// data layer for Sprint 2's generic alert engine: previously `internal/alerts`
// was a purely in-memory threshold engine with no persistence, no API and no
// UI.
type Repository struct{ DB *pgxpool.Pool }

// PersistedRule is a `alert_rules` row. Condition is the parsed condition_config
// JSONB (e.g. {"metric":"icmp_loss_pct","operator":">","threshold":30,"unit":"%"}).
type PersistedRule struct {
	ID                    int64          `json:"id"`
	Name                  string         `json:"name"`
	Description           string         `json:"description"`
	RuleType              string         `json:"ruleType"`
	Condition             map[string]any `json:"condition"`
	Severity              string         `json:"severity"`
	ForDurationSec        int            `json:"forDurationSec"`
	CooldownSec           int            `json:"cooldownSec"`
	NotificationChannelIDs []int64        `json:"notificationChannelIds"`
	DeviceGroup           string         `json:"deviceGroup"`
	Enabled               bool           `json:"enabled"`
	CreatedBy             *int64         `json:"createdBy,omitempty"`
	CreatedAt             time.Time      `json:"createdAt"`
	UpdatedAt             time.Time      `json:"updatedAt"`
}

// PersistedChannel is a `notification_channels` row. Config holds arbitrary
// per-type settings (webhook URL, slack webhook, email recipients, ...).
type PersistedChannel struct {
	ID          int64          `json:"id"`
	TenantID    string         `json:"tenantId"`
	Name        string         `json:"name"`
	ChannelType string         `json:"channelType"`
	Config      map[string]any `json:"config"`
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"createdAt"`
}

const defaultSeverity = "warning"

// ListRules returns all alert rules ordered by id.
func (r Repository) ListRules(ctx context.Context) ([]PersistedRule, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("alerts repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,name,description,rule_type,condition_config,severity,
		for_duration_sec,cooldown_sec,notification_channel_ids,device_group,is_enabled,created_by,created_at,updated_at
		FROM alert_rules ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersistedRule{}
	for rows.Next() {
		rule := PersistedRule{Condition: map[string]any{}}
		var cond []byte
		var channels []byte
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.RuleType, &cond,
			&rule.Severity, &rule.ForDurationSec, &rule.CooldownSec, &channels,
			&rule.DeviceGroup, &rule.Enabled, &rule.CreatedBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(cond, &rule.Condition)
		_ = json.Unmarshal(channels, &rule.NotificationChannelIDs)
		out = append(out, rule)
	}
	return out, rows.Err()
}

// SaveRule inserts a new rule and returns it with its generated id.
func (r Repository) SaveRule(ctx context.Context, rule PersistedRule) (PersistedRule, error) {
	if r.DB == nil {
		return PersistedRule{}, fmt.Errorf("alerts repository is not initialized")
	}
	if strings.TrimSpace(rule.Name) == "" {
		return PersistedRule{}, fmt.Errorf("rule name is required")
	}
	if rule.RuleType == "" {
		rule.RuleType = "threshold"
	}
	if rule.Severity == "" {
		rule.Severity = defaultSeverity
	}
	if rule.CooldownSec <= 0 {
		rule.CooldownSec = 300
	}
	cond, err := json.Marshal(rule.Condition)
	if err != nil {
		return PersistedRule{}, err
	}
	channels, err := json.Marshal(rule.NotificationChannelIDs)
	if err != nil {
		return PersistedRule{}, err
	}
	err = r.DB.QueryRow(ctx, `INSERT INTO alert_rules
		(name,description,rule_type,condition_config,severity,for_duration_sec,cooldown_sec,notification_channel_ids,device_group,is_enabled,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id,created_at,updated_at`,
		rule.Name, rule.Description, rule.RuleType, cond, rule.Severity,
		rule.ForDurationSec, rule.CooldownSec, channels, rule.DeviceGroup, rule.Enabled, rule.CreatedBy).
		Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return PersistedRule{}, err
	}
	if rule.DeviceGroup == "" {
		rule.DeviceGroup = "all"
	}
	return rule, nil
}

// UpdateRule updates the mutable fields of a rule by id.
func (r Repository) UpdateRule(ctx context.Context, id int64, rule PersistedRule) (PersistedRule, error) {
	if r.DB == nil {
		return PersistedRule{}, fmt.Errorf("alerts repository is not initialized")
	}
	if strings.TrimSpace(rule.Name) == "" {
		return PersistedRule{}, fmt.Errorf("rule name is required")
	}
	if rule.Severity == "" {
		rule.Severity = defaultSeverity
	}
	cond, err := json.Marshal(rule.Condition)
	if err != nil {
		return PersistedRule{}, err
	}
	channels, err := json.Marshal(rule.NotificationChannelIDs)
	if err != nil {
		return PersistedRule{}, err
	}
	err = r.DB.QueryRow(ctx, `UPDATE alert_rules SET
		name=$2, description=$3, rule_type=$4, condition_config=$5, severity=$6,
		for_duration_sec=$7, cooldown_sec=$8, notification_channel_ids=$9, device_group=$10, is_enabled=$11,
		updated_at=NOW()
		WHERE id=$1 RETURNING id,created_at,updated_at`,
		id, rule.Name, rule.Description, rule.RuleType, cond, rule.Severity,
		rule.ForDurationSec, rule.CooldownSec, channels, rule.DeviceGroup, rule.Enabled).
		Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return PersistedRule{}, err
	}
	if rule.DeviceGroup == "" {
		rule.DeviceGroup = "all"
	}
	return rule, nil
}

// SetRuleEnabled toggles a rule without rewriting its condition.
func (r Repository) SetRuleEnabled(ctx context.Context, id int64, enabled bool) error {
	if r.DB == nil {
		return fmt.Errorf("alerts repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE alert_rules SET is_enabled=$2, updated_at=NOW() WHERE id=$1`, id, enabled)
	return err
}

// DeleteRule removes a rule by id.
func (r Repository) DeleteRule(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("alerts repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM alert_rules WHERE id=$1`, id)
	return err
}

// ListChannels returns all notification channels for a tenant (or all if
// tenantId is empty), ordered by id.
func (r Repository) ListChannels(ctx context.Context, tenantID string) ([]PersistedChannel, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("alerts repository is not initialized")
	}
	var rows pgx.Rows
	var err error
	if tenantID == "" {
		rows, err = r.DB.Query(ctx, `SELECT id,tenant_id,name,channel_type,config,is_enabled,created_at FROM notification_channels ORDER BY id ASC`)
	} else {
		rows, err = r.DB.Query(ctx, `SELECT id,tenant_id,name,channel_type,config,is_enabled,created_at FROM notification_channels WHERE tenant_id=$1 ORDER BY id ASC`, tenantID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersistedChannel{}
	for rows.Next() {
		ch := PersistedChannel{Config: map[string]any{}}
		var cfg []byte
		if err := rows.Scan(&ch.ID, &ch.TenantID, &ch.Name, &ch.ChannelType, &cfg, &ch.Enabled, &ch.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(cfg, &ch.Config)
		out = append(out, ch)
	}
	return out, rows.Err()
}

// SaveChannel inserts a new notification channel and returns it with its id.
func (r Repository) SaveChannel(ctx context.Context, ch PersistedChannel) (PersistedChannel, error) {
	if r.DB == nil {
		return PersistedChannel{}, fmt.Errorf("alerts repository is not initialized")
	}
	if strings.TrimSpace(ch.Name) == "" {
		return PersistedChannel{}, fmt.Errorf("channel name is required")
	}
	if ch.ChannelType == "" {
		return PersistedChannel{}, fmt.Errorf("channel type is required")
	}
	cfg, err := json.Marshal(ch.Config)
	if err != nil {
		return PersistedChannel{}, err
	}
	err = r.DB.QueryRow(ctx, `INSERT INTO notification_channels (tenant_id,name,channel_type,config,is_enabled)
		VALUES ($1,$2,$3,$4,$5) RETURNING id,created_at`,
		ch.TenantID, ch.Name, ch.ChannelType, cfg, ch.Enabled).Scan(&ch.ID, &ch.CreatedAt)
	if err != nil {
		return PersistedChannel{}, err
	}
	return ch, nil
}

// DeleteChannel removes a notification channel by id.
func (r Repository) DeleteChannel(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("alerts repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM notification_channels WHERE id=$1`, id)
	return err
}

