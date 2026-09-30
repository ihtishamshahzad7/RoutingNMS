package ifpoll

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// HistoryPoint is one sample in a port's history-range response: the same
// timestamp always carries OperUp (recorded every poll cycle), and
// InRateBps/OutRateBps when that cycle had a prior sample to diff against
// (see poller.go's pollDevice -- a nil rate here means "no data point",
// not "zero traffic").
type HistoryPoint struct {
	ProbedAt   string   `json:"probedAt"`
	InRateBps  *float64 `json:"inRateBps,omitempty"`
	OutRateBps *float64 `json:"outRateBps,omitempty"`
	OperUp     bool     `json:"operUp"`
}

// HistoryAPI backs GET /api/v1/interfaces/{id}/history-range?range=1h|24h|7d
// -- item 3.4's per-port history view on the device detail page. {id} is
// the port's stable interfaces.id (the same id the Ports table and the
// Events page's highlight deep-link both key on), not device_id/if_index.
type HistoryAPI struct{ Metrics metricsdb.Repository }

func (a HistoryAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := historyPathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()
	var series []metricsdb.Series
	var err error
	// Explicit ?from=&to= (RFC3339) wins over the range preset -- same
	// pattern item 2.4 added to ping/portcheck's history-range endpoints,
	// extended here (item 3.6) so the Download Report PDF can include port
	// up/down history for an arbitrary past date range on SNMP-enabled
	// devices, not just a 1h/24h/7d window ending "now".
	if fromStr, toStr := r.URL.Query().Get("from"), r.URL.Query().Get("to"); fromStr != "" && toStr != "" {
		from, err1 := time.Parse(time.RFC3339, fromStr)
		to, err2 := time.Parse(time.RFC3339, toStr)
		if err1 != nil || err2 != nil {
			http.Error(w, "from/to must be RFC3339 timestamps", http.StatusBadRequest)
			return
		}
		series, err = a.Metrics.QueryBetween(ctx, "interface", id, []string{"if_oper_up", "if_in_rate_bps", "if_out_rate_bps"}, from, to)
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
		series, err = a.Metrics.Query(ctx, "interface", id, []string{"if_oper_up", "if_in_rate_bps", "if_out_rate_bps"}, window)
	}
	if err != nil {
		http.Error(w, "failed to load port history", http.StatusInternalServerError)
		return
	}

	// The oper-up series is the backbone -- it's written every poll cycle,
	// unlike the rate series which skip a port's first poll (and any
	// counter-reset cycle). In/out rate points are matched onto it by
	// exact recorded_at, which is safe because both are written from the
	// same RecordBatch call using one shared `now` per cycle (see
	// poller.go), so a real match is always an exact timestamp match, not
	// a nearest-neighbor guess.
	var operSeries, inSeries, outSeries metricsdb.Series
	for _, s := range series {
		switch s.Metric {
		case "if_oper_up":
			operSeries = s
		case "if_in_rate_bps":
			inSeries = s
		case "if_out_rate_bps":
			outSeries = s
		}
	}
	inByTS := map[int64]float64{}
	for _, p := range inSeries.Points {
		inByTS[p.Timestamp.UnixNano()] = p.Value
	}
	outByTS := map[int64]float64{}
	for _, p := range outSeries.Points {
		outByTS[p.Timestamp.UnixNano()] = p.Value
	}

	history := make([]HistoryPoint, 0, len(operSeries.Points))
	for _, p := range operSeries.Points {
		hp := HistoryPoint{ProbedAt: p.Timestamp.Format(time.RFC3339), OperUp: p.Value >= 0.5}
		if v, ok := inByTS[p.Timestamp.UnixNano()]; ok {
			hp.InRateBps = &v
		}
		if v, ok := outByTS[p.Timestamp.UnixNano()]; ok {
			hp.OutRateBps = &v
		}
		history = append(history, hp)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"history": history})
}

// historyPathID extracts {id} from /api/v1/interfaces/{id}/history-range,
// mirroring ping/portcheck's own pathID helper (the id is the
// second-to-last path segment).
func historyPathID(r *http.Request) (string, bool) {
	parts := []string{}
	cur := ""
	for i := 0; i < len(r.URL.Path); i++ {
		if r.URL.Path[i] == '/' {
			if cur != "" {
				parts = append(parts, cur)
				cur = ""
			}
			continue
		}
		cur += string(r.URL.Path[i])
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	if len(parts) < 3 || !strings.EqualFold(parts[len(parts)-1], "history-range") {
		return "", false
	}
	return parts[len(parts)-2], true
}
