package maintenance

// report.go: computing maintenance-window overlap for a historical date
// range, backing the Download Report PDF (6-page rebuild, item 2.4) --
// "if a check was in a maintenance/paused state during the range, note
// that separately from real downtime". Checker.ActiveSubjects only
// answers "right now"; IntervalsForSubject answers "which time spans
// within [from,to] were covered", reusing the same single/recurring
// evaluation rules as activeWindowRow.covers, just enumerated over a
// range instead of tested against one instant.

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Interval is one covered time span, clipped to the requested [from,to].
type Interval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Title string    `json:"title"`
}

// intervalRow mirrors activeWindowRow, adding Title for report display.
type intervalRow struct {
	Title           string
	Strategy        string
	StartsAt        *time.Time
	EndsAt          *time.Time
	DaysOfWeek      []int
	StartTimeOfDay  *time.Time
	DurationMinutes int
	Timezone        string
}

// IntervalsForSubject returns every maintenance-window interval that
// overlaps [from,to] for the given subject (subjectType "device"|"olt"),
// merged and clipped to that range. Only windows explicitly assigned to
// the subject and marked active are considered.
func IntervalsForSubject(ctx context.Context, db *pgxpool.Pool, subjectType, subjectID string, from, to time.Time) ([]Interval, error) {
	out := []Interval{}
	if db == nil {
		return out, nil
	}
	rows, err := db.Query(ctx, `
		SELECT w.title, w.strategy, w.starts_at, w.ends_at, w.days_of_week, w.start_time_of_day, w.duration_minutes, w.timezone
		FROM maintenance_windows w
		JOIN maintenance_window_items i ON i.maintenance_window_id = w.id
		WHERE w.active = true AND i.subject_type = $1 AND i.subject_id = $2`, subjectType, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var windows []intervalRow
	for rows.Next() {
		var w intervalRow
		if err := rows.Scan(&w.Title, &w.Strategy, &w.StartsAt, &w.EndsAt, &w.DaysOfWeek, &w.StartTimeOfDay, &w.DurationMinutes, &w.Timezone); err != nil {
			return nil, err
		}
		windows = append(windows, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, w := range windows {
		switch w.Strategy {
		case "single":
			if w.StartsAt == nil || w.EndsAt == nil {
				continue
			}
			start, end := *w.StartsAt, *w.EndsAt
			if end.Before(from) || start.After(to) {
				continue
			}
			if start.Before(from) {
				start = from
			}
			if end.After(to) {
				end = to
			}
			out = append(out, Interval{Start: start, End: end, Title: w.Title})

		case "recurring":
			if w.StartTimeOfDay == nil || w.DurationMinutes <= 0 || len(w.DaysOfWeek) == 0 {
				continue
			}
			loc, err := time.LoadLocation(w.Timezone)
			if err != nil {
				loc = time.UTC
			}
			dayMatch := map[int]bool{}
			for _, d := range w.DaysOfWeek {
				dayMatch[d] = true
			}
			// Walk each calendar day touching [from,to] (inclusive, with a
			// one-day pad on each side so an occurrence starting the day
			// before `from` but running past midnight into the range is
			// still caught) and, for days matching daysOfWeek, compute
			// that occurrence's [start,end) in the window's timezone.
			cursor := from.In(loc).AddDate(0, 0, -1)
			last := to.In(loc).AddDate(0, 0, 1)
			for !cursor.After(last) {
				if dayMatch[int(cursor.Weekday())] {
					start := time.Date(cursor.Year(), cursor.Month(), cursor.Day(),
						w.StartTimeOfDay.Hour(), w.StartTimeOfDay.Minute(), w.StartTimeOfDay.Second(), 0, loc)
					end := start.Add(time.Duration(w.DurationMinutes) * time.Minute)
					if !end.Before(from) && !start.After(to) {
						clippedStart, clippedEnd := start, end
						if clippedStart.Before(from) {
							clippedStart = from
						}
						if clippedEnd.After(to) {
							clippedEnd = to
						}
						out = append(out, Interval{Start: clippedStart, End: clippedEnd, Title: w.Title})
					}
				}
				cursor = cursor.AddDate(0, 0, 1)
			}
		}
	}
	return out, nil
}

// IntervalsAPI backs GET /api/v1/maintenance-windows/for-device/{id}
// ?from=&to= (RFC3339) -- read-only, used by the Download Report PDF
// (item 2.4) to shade/annotate maintenance coverage over the selected
// date range. Session-authed like the rest of the maintenance-windows API.
type IntervalsAPI struct{ DB *pgxpool.Pool }

func (a IntervalsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	fromStr, toStr := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	from, err1 := time.Parse(time.RFC3339, fromStr)
	to, err2 := time.Parse(time.RFC3339, toStr)
	if err1 != nil || err2 != nil {
		http.Error(w, "from/to must be RFC3339 timestamps", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	intervals, err := IntervalsForSubject(ctx, a.DB, "device", id, from, to)
	if err != nil {
		http.Error(w, "failed to load maintenance intervals", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"intervals": intervals})
}
