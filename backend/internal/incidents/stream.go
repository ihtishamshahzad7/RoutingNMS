package incidents

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Stream struct { mu sync.RWMutex; clients map[chan Incident]struct{} }
func NewStream() *Stream { return &Stream{clients:map[chan Incident]struct{}{}} }
func (s *Stream) Publish(i Incident) { s.mu.RLock(); defer s.mu.RUnlock(); for ch := range s.clients { select { case ch <- i: default: } } }

// ServeHTTP implements Server-Sent Events for the NOC dashboard. SSE keeps the
// browser connection simple while allowing instant incident updates.
func (s *Stream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type","text/event-stream"); w.Header().Set("Cache-Control","no-cache"); w.Header().Set("Connection","keep-alive")
	flusher,ok:=w.(http.Flusher); if !ok { http.Error(w,"streaming unsupported",500); return }
	ch:=make(chan Incident,16); s.mu.Lock(); s.clients[ch]=struct{}{}; s.mu.Unlock(); defer func(){s.mu.Lock();delete(s.clients,ch);close(ch);s.mu.Unlock()}()
	for { select { case <-r.Context().Done(): return; case i:=<-ch: b,_:=json.Marshal(i); _,_=w.Write([]byte("event: incident\ndata: "+string(b)+"\n\n")); flusher.Flush() } }
}
