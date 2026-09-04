// Package maintenance implements maintenance windows, ported from Uptime
// Kuma: an operator can schedule planned downtime for chosen devices/OLTs
// (a one-off window, or a weekly recurring one) so that a scheduled truck
// roll or a firmware upgrade doesn't fire alerts or wake anyone up.
package maintenance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Window struct {
	ID              int64      `json:"id"`
	TenantID        string     `json:"tenantId"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Strategy        string     `json:"strategy"` // "single" | "recurring"
	StartsAt        *time.Time `json:"startsAt,omitempty"`
	EndsAt          *time.Time `json:"endsAt,omitempty"`
	DaysOfWeek      []int      `json:"daysOfWeek,omitempty"`     // 0=Sunday..6=Saturday, recurring only
	StartTimeOfDay  *string    `json:"startTimeOfDay,omitempty"` // "HH:MM[:SS]", recurring only
	DurationMinutes int        `json:"durationMinutes"`
	Timezone        string     `json:"timezone"`
	Active          bool       `json:"active"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type Item struct {
	ID          int64  `json:"id"`
	WindowID    int64  `json:"maintenanceWindowId"`
	SubjectType string `json:"subjectType"` // "device" | "olt"
	SubjectID   string `json:"subjectId"`
}

type Repository struct{ DB *pgxpool.Pool }

func validate(w Window) error {
	if strings.TrimSpace(w.Title) == "" {
		return fmt.Errorf("title is required")
	}
	switch w.Strategy {
	case "single":
		if w.StartsAt == nil || w.EndsAt == nil {
			return fmt.Errorf("single-strategy windows require startsAt and endsAt")
		}
		if !w.EndsAt.After(*w.StartsAt) {
			return fmt.Errorf("endsAt must be after startsAt")
		}
	case "recurring":
		if len(w.DaysOfWeek) == 0 {
			return fmt.Errorf("recurring-strategy windows require at least one day of week")
		}
		for _, d := range w.DaysOfWeek {
			if d < 0 || d > 6 {
				return fmt.Errorf("daysOfWeek must be 0 (Sunday) through 6 (Saturday)")
			}
		}
		if w.StartTimeOfDay == nil || strings.TrimSpace(*w.StartTimeOfDay) == "" {
			return fmt.Errorf("recurring-strategy windows require startTimeOfDay")
		}
		if w.DurationMinutes <= 0 {
			return fmt.Errorf("durationMinutes must be positive")
		}
	default:
		return fmt.Errorf("strategy must be \"single\" or \"recurring\"")
	}
	if w.Timezone == "" {
		w.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(w.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q", w.Timezone)
	}
	return nil
}

const selectCols = `id,tenant_id,title,description,strategy,starts_at,ends_at,days_of_week,start_time_of_day,duration_minutes,timezone,active,created_at,updated_at`

func scanWindow(row interface{ Scan(...any) error }) (Window, error) {
	var w Window
	var startTOD *time.Time
	err := row.Scan(&w.ID, &w.TenantID, &w.Title, &w.Description, &w.Strategy, &w.StartsAt, &w.EndsAt,
		&w.DaysOfWeek, &startTOD, &w.DurationMinutes, &w.Timezone, &w.Active, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return Window{}, err
	}
	if startTOD != nil {
		s := startTOD.Format("15:04:05")
		w.StartTimeOfDay = &s
	}
	return w, nil
}

func (r Repository) List(ctx context.Context, tenantID string) ([]Window, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("maintenance repository is not initialized")
	}
	query := `SELECT ` + selectCols + ` FROM maintenance_windows ORDER BY title`
	args := []any{}
	if tenantID != "" {
		query = `SELECT ` + selectCols + ` FROM maintenance_windows WHERE tenant_id=$1 ORDER BY title`
		args = append(args, tenantID)
	}
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Window{}
	for rows.Next() {
		w, err := scanWindow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r Repository) Get(ctx context.Context, id int64) (Window, error) {
	if r.DB == nil {
		return Window{}, fmt.Errorf("maintenance repository is not initialized")
	}
	row := r.DB.QueryRow(ctx, `SELECT `+selectCols+` FROM maintenance_windows WHERE id=$1`, id)
	return scanWindow(row)
}

func (r Repository) Create(ctx context.Context, w Window) (Window, error) {
	if r.DB == nil {
		return Window{}, fmt.Errorf("maintenance repository is not initialized")
	}
	if w.Timezone == "" {
		w.Timezone = "UTC"
	}
	if err := validate(w); err != nil {
		return Window{}, err
	}
	row := r.DB.QueryRow(ctx, `INSERT INTO maintenance_windows
		(tenant_id,title,description,strategy,starts_at,ends_at,days_of_week,start_time_of_day,duration_minutes,timezone,active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING `+selectCols,
		w.TenantID, w.Title, w.Description, w.Strategy, w.StartsAt, w.EndsAt, w.DaysOfWeek, w.StartTimeOfDay, w.DurationMinutes, w.Timezone, w.Active)
	return scanWindow(row)
}

func (r Repository) Update(ctx context.Context, id int64, w Window) (Window, error) {
	if r.DB == nil {
		return Window{}, fmt.Errorf("maintenance repository is not initialized")
	}
	if w.Timezone == "" {
		w.Timezone = "UTC"
	}
	if err := validate(w); err != nil {
		return Window{}, err
	}
	row := r.DB.QueryRow(ctx, `UPDATE maintenance_windows SET
		title=$2,description=$3,strategy=$4,starts_at=$5,ends_at=$6,days_of_week=$7,start_time_of_day=$8,duration_minutes=$9,timezone=$10,active=$11,updated_at=NOW()
		WHERE id=$1 RETURNING `+selectCols,
		id, w.Title, w.Description, w.Strategy, w.StartsAt, w.EndsAt, w.DaysOfWeek, w.StartTimeOfDay, w.DurationMinutes, w.Timezone, w.Active)
	return scanWindow(row)
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("maintenance repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM maintenance_windows WHERE id=$1`, id)
	return err
}

func (r Repository) ListItems(ctx context.Context, windowID int64) ([]Item, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("maintenance repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,maintenance_window_id,subject_type,subject_id FROM maintenance_window_items WHERE maintenance_window_id=$1 ORDER BY id`, windowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.WindowID, &it.SubjectType, &it.SubjectID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReplaceItems atomically replaces the full assigned-subjects list for a
// window -- the admin UI always submits the complete set, not incremental
// add/remove operations.
func (r Repository) ReplaceItems(ctx context.Context, windowID int64, items []Item) error {
	if r.DB == nil {
		return fmt.Errorf("maintenance repository is not initialized")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM maintenance_window_items WHERE maintenance_window_id=$1`, windowID); err != nil {
		return err
	}
	for i, it := range items {
		if it.SubjectType != "device" && it.SubjectType != "olt" {
			return fmt.Errorf("item %d: subjectType must be \"device\" or \"olt\"", i)
		}
		if strings.TrimSpace(it.SubjectID) == "" {
			return fmt.Errorf("item %d: subjectId is required", i)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO maintenance_window_items (maintenance_window_id,subject_type,subject_id) VALUES ($1,$2,$3)`,
			windowID, it.SubjectType, it.SubjectID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// activeWindowRow is what's needed from the DB to evaluate whether a window
// currently covers "now", for every subject assigned to it.
type activeWindowRow struct {
	ID              int64
	Strategy        string
	StartsAt        *time.Time
	EndsAt          *time.Time
	DaysOfWeek      []int
	StartTimeOfDay  *time.Time
	DurationMinutes int
	Timezone        string
}

// covers reports whether this window is in effect at instant `now`.
func (w activeWindowRow) covers(now time.Time) bool {
	switch w.Strategy {
	case "single":
		if w.StartsAt == nil || w.EndsAt == nil {
			return false
		}
		return !now.Before(*w.StartsAt) && !now.After(*w.EndsAt)
	case "recurring":
		if w.StartTimeOfDay == nil || w.DurationMinutes <= 0 {
			return false
		}
		loc, err := time.LoadLocation(w.Timezone)
		if err != nil {
			loc = time.UTC
		}
		local := now.In(loc)
		dow := int(local.Weekday())
		matchesDay := false
		for _, d := range w.DaysOfWeek {
			if d == dow {
				matchesDay = true
				break
			}
		}
		// A window that started yesterday (local) and runs past midnight
		// still needs checking against yesterday's weekday too.
		if !matchesDay {
			yesterday := local.AddDate(0, 0, -1)
			for _, d := range w.DaysOfWeek {
				if d == int(yesterday.Weekday()) {
					startOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(),
						w.StartTimeOfDay.Hour(), w.StartTimeOfDay.Minute(), w.StartTimeOfDay.Second(), 0, loc)
					end := startOfYesterday.Add(time.Duration(w.DurationMinutes) * time.Minute)
					if !local.Before(startOfYesterday) && local.Before(end) {
						return true
					}
				}
			}
			return false
		}
		start := time.Date(local.Year(), local.Month(), local.Day(),
			w.StartTimeOfDay.Hour(), w.StartTimeOfDay.Minute(), w.StartTimeOfDay.Second(), 0, loc)
		end := start.Add(time.Duration(w.DurationMinutes) * time.Minute)
		return !local.Before(start) && local.Before(end)
	default:
		return false
	}
}

// Checker answers "is this subject currently under an active maintenance
// window" -- the integration point consumed by internal/alertsfeed to
// suppress alerts during planned downtime.
type Checker struct{ DB *pgxpool.Pool }

// ActiveSubjects returns the set of (subjectType,subjectId) pairs -- keyed as
// "device:123" / "olt:45" -- that are currently covered by an active
// maintenance window. Evaluated in Go rather than SQL so the recurring-window
// timezone/day-of-week/duration logic only needs to live in one place.
func (c Checker) ActiveSubjects(ctx context.Context) (map[string]bool, error) {
	out := map[string]bool{}
	if c.DB == nil {
		return out, nil
	}
	rows, err := c.DB.Query(ctx, `
		SELECT w.id, w.strategy, w.starts_at, w.ends_at, w.days_of_week, w.start_time_of_day, w.duration_minutes, w.timezone,
		       i.subject_type, i.subject_id
		FROM maintenance_windows w
		JOIN maintenance_window_items i ON i.maintenance_window_id = w.id
		WHERE w.active = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	for rows.Next() {
		var w activeWindowRow
		var subjectType, subjectID string
		if err := rows.Scan(&w.ID, &w.Strategy, &w.StartsAt, &w.EndsAt, &w.DaysOfWeek, &w.StartTimeOfDay, &w.DurationMinutes, &w.Timezone, &subjectType, &subjectID); err != nil {
			return nil, err
		}
		if w.covers(now) {
			out[subjectType+":"+subjectID] = true
		}
	}
	return out, rows.Err()
}
