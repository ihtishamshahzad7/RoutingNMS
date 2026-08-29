package metrics

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Sample struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Labels    map[string]string
}

type Writer struct {
	BaseURL string
	Client  *http.Client
}

func (w Writer) Write(ctx context.Context, samples []Sample) error {
	if w.BaseURL == "" || len(samples) == 0 { return nil }
	var body bytes.Buffer
	for _, s := range samples {
		body.WriteString(s.Name)
		if len(s.Labels) > 0 {
			body.WriteByte('{')
			first := true
			for k, v := range s.Labels {
				if !first { body.WriteByte(',') }; first = false
				body.WriteString(k); body.WriteString("=\""); body.WriteString(escape(v)); body.WriteString("\"")
			}
			body.WriteByte('}')
		}
		body.WriteByte(' '); body.WriteString(strconv.FormatFloat(s.Value, 'f', -1, 64))
		body.WriteByte(' '); body.WriteString(strconv.FormatInt(s.Timestamp.UnixMilli(), 10)); body.WriteByte('\n')
	}
	client := w.Client; if client == nil { client = &http.Client{Timeout: 10 * time.Second} }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(w.BaseURL, "/")+"/api/v1/import/prometheus", &body)
	if err != nil { return err }
	req.Header.Set("Content-Type", "text/plain")
	resp, err := client.Do(req); if err != nil { return fmt.Errorf("victoriametrics write: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 { return fmt.Errorf("victoriametrics write returned HTTP %d", resp.StatusCode) }
	return nil
}

func escape(value string) string { return strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n").Replace(value) }
