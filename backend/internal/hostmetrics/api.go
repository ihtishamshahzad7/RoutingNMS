package hostmetrics

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// HistoryPoint is one sample in a device's CPU/memory history-range
// response. CPUPercent/MemoryPercent are independently optional (unlike
// item 3.4's port history, where oper-up is always present as a backbone,
// neither metric here is guaranteed present on every cycle -- a device
// that only exposes hrProcessorLoad but no usable hrStorage RAM row, for
// example) -- a nil field means "no data point here", not zero.
type HistoryPoint struct {
	ProbedAt      string   `json:"probedAt"`
	CPUPercent    *float64 `json:"cpuPercent,omitempty"`
	MemoryPercent *float64 `json:"memoryPercent,omitempty"`
}

// HistoryAPI backs GET /api/v1/devices/{id}/host-metrics/history-range
// ?range=1h|24h|7d -- item 3.5's CPU/memory graph on the device detail
// page, using the same {id}-path-parameter routing style as this
// project's newer device sub-routes (e.g. internal/configbackup), rather
// than the manual suffix parsing item 3.1-3.4's endpoints used.
type HistoryAPI struct{ Metrics metricsdb.Repository }

func (a HistoryAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "device ID is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var series []metricsdb.Series
	var err error
	// Explicit ?from=&to= (RFC3339) wins over the range preset -- same
	// pattern item 2.4 added to ping/portcheck, extended here (item 3.6) so
	// the Download Report PDF can include CPU/memory history/threshold
	// events for an arbitrary past date range on SNMP-enabled devices.
	if fromStr, toStr := r.URL.Query().Get("from"), r.URL.Query().Get("to"); fromStr != "" && toStr != "" {
		from, err1 := time.Parse(time.RFC3339, fromStr)
		to, err2 := time.Parse(time.RFC3339, toStr)
		if err1 != nil || err2 != nil {
			http.Error(w, "from/to must be RFC3339 timestamps", http.StatusBadRequest)
			return
		}
		series, err = a.Metrics.QueryBetween(ctx, "device", id, []string{"cpu_percent", "memory_percent"}, from, to)
	} else {
		var window time.Duration
		switch r.URL.Query().Get("range") {
		case "1h":
			window = time.Hour
		case "7d":
			window = 7 * 24 * time.Hour
		default:
			window = 24 * time.Hour
		}
		series, err = a.Metrics.Query(ctx, "device", id, []string{"cpu_percent", "memory_percent"}, window)
	}
	if err != nil {
		http.Error(w, "failed to load host metrics history", http.StatusInternalServerError)
		return
	}

	var cpuSeries, memSeries metricsdb.Series
	for _, s := range series {
		switch s.Metric {
		case "cpu_percent":
			cpuSeries = s
		case "memory_percent":
			memSeries = s
		}
	}

	// Neither series is guaranteed present every cycle, so the response is
	// built from the union of both series' timestamps rather than treating
	// one as a backbone.
	type point struct {
		ts  time.Time
		cpu *float64
		mem *float64
	}
	byTS := map[int64]*point{}
	order := []int64{}
	upsert := func(ts time.Time, apply func(*point)) {
		key := ts.UnixNano()
		p, ok := byTS[key]
		if !ok {
			p = &point{ts: ts}
			byTS[key] = p
			order = append(order, key)
		}
		apply(p)
	}
	for _, p := range cpuSeries.Points {
		v := p.Value
		upsert(p.Timestamp, func(pt *point) { pt.cpu = &v })
	}
	for _, p := range memSeries.Points {
		v := p.Value
		upsert(p.Timestamp, func(pt *point) { pt.mem = &v })
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	history := make([]HistoryPoint, 0, len(order))
	for _, key := range order {
		p := byTS[key]
		history = append(history, HistoryPoint{ProbedAt: p.ts.Format(time.RFC3339), CPUPercent: p.cpu, MemoryPercent: p.mem})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"history": history})
}
