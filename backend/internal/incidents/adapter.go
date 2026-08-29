package incidents

// AlertSink is the small contract vendor and monitoring modules use to emit
// normalized alerts. The concrete incident engine remains replaceable.
type AlertSink interface { Emit(Alert) (Incident, error) }

type EngineSink struct { Correlator Correlator }

func (s EngineSink) Emit(a Alert) (Incident, error) { return s.Correlator.Process(a) }

// EmitMany preserves each alert's resource identity while allowing a poller
// to submit a batch without knowing anything about incident storage.
func EmitMany(s AlertSink, alerts []Alert) ([]Incident, error) {
	out := make([]Incident, 0, len(alerts))
	for _, a := range alerts {
		i, err := s.Emit(a)
		if err != nil { return nil, err }
		out = append(out, i)
	}
	return out, nil
}
