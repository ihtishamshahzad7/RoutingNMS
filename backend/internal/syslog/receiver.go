// Package syslog implements a syslog receiver (RFC 3164 / BSD-style, and
// RFC 5424) so OLTs, routers, switches, CMTS gear and RouterOS/IOS devices
// in the fleet can point their "syslog server" setting at this NMS. Parsing
// is deliberately best-effort: a line that doesn't fully match either RFC
// still gets stored with the full raw text, rather than being dropped,
// because ISP access-layer gear is inconsistent about how closely it
// follows either spec.
package syslog

import (
	"bufio"
	"context"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// priRe matches the leading "<PRI>" of a syslog message, e.g. "<134>".
// PRI = facility*8 + severity, per RFC 3164 section 4.1.1 (RFC 5424 uses
// the same PRI encoding, section 6.2.1).
var priRe = regexp.MustCompile(`^<(\d{1,3})>`)

// rfc5424VersionRe matches the "1 " VERSION token that immediately follows
// PRI in an RFC 5424 message (RFC 3164 has no such token, so its absence is
// how we tell the two formats apart).
var rfc5424VersionRe = regexp.MustCompile(`^1 `)

// tagRe pulls a best-effort "tag" (process/program name) off the front of
// the message body, e.g. "sshd[1234]: " or "OLT_ALARM: ".
var tagRe = regexp.MustCompile(`^(\S+?):\s?`)

// ciscoSeqRe strips a leading Cisco "service sequence-numbers" counter,
// e.g. "000123: ", which would otherwise be mistaken for part of the
// hostname/timestamp fields.
var ciscoSeqRe = regexp.MustCompile(`^\d+:\s+`)

// ciscoMnemonicRe matches Cisco IOS's "%FACILITY-SEVERITY-MNEMONIC:" message
// marker, e.g. "%LINK-3-UPDOWN:". When present it's a much more useful "tag"
// than the generic colon-terminated word tagRe would otherwise grab.
var ciscoMnemonicRe = regexp.MustCompile(`^(%[A-Z0-9_]+-[0-7]-[A-Z0-9_]+):\s?`)

// bsdTimestampRe matches an RFC 3164 BSD timestamp plus the hostname that
// follows it: "Mon Jan 2 15:04:05 hostname rest...". A regex (rather than a
// naive space-split) is used deliberately -- the traditional BSD format
// pads a single-digit day with an extra space ("Jan  2", two spaces), which
// a fixed-field split misparses; \s+ absorbs any amount of padding. Also
// tolerates Cisco's leading "*" (uncertain clock) and optional ".123"
// milliseconds on the time field.
var bsdTimestampRe = regexp.MustCompile(`^\*?(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+(\d{1,2})\s+(\d{2}:\d{2}:\d{2})(?:\.\d+)?\s+(\S+)\s?(.*)$`)

// MaxMessageBytes caps how much of a single line/datagram is kept. Longer
// lines are truncated (with a marker appended) rather than rejected outright
// -- the point is to bound memory/storage per message, not to drop data a
// device sends.
const MaxMessageBytes = 4096

type Message struct {
	SourceIP  string
	Facility  int
	Severity  int
	Hostname  string
	Tag       string
	Body      string
	// DeviceTimestamp is the message's own timestamp, when one could be
	// confidently parsed. Nil when the message had no timestamp, or its
	// timestamp couldn't be parsed -- callers should fall back to the
	// receive time in that case (see TimestampEstimated).
	DeviceTimestamp *time.Time
	// TimestampEstimated is true whenever DeviceTimestamp is nil, i.e. the
	// stored message_timestamp will actually be the receive time rather
	// than something the device reported.
	TimestampEstimated bool
}

// Parse extracts PRI (facility/severity), hostname, a best-effort tag, and
// the message body from a raw syslog line, trying RFC 5424 first (it has an
// unambiguous "<PRI>1 " marker) and falling back to RFC 3164/BSD-style
// parsing -- which also covers MikroTik RouterOS's default remote-logging
// format (BSD framing with a comma-separated "topic,info" section in place
// of a traditional "tag[pid]:") and Cisco IOS's "%FACILITY-SEVERITY-
// MNEMONIC:" style, both handled below. Anything that can't be confidently
// parsed still comes back with the full original text in Body, so nothing
// is silently dropped. receivedAt supplies the year for RFC 3164's
// year-less timestamp and is used verbatim as DeviceTimestamp is left nil.
func Parse(sourceIP string, raw string, receivedAt time.Time) Message {
	if len(raw) > MaxMessageBytes {
		raw = raw[:MaxMessageBytes] + " …(truncated)"
	}

	m := Message{SourceIP: sourceIP, Facility: -1, Severity: -1, Body: raw, TimestampEstimated: true}
	rest := raw
	if loc := priRe.FindStringSubmatchIndex(rest); loc != nil {
		if pri, err := strconv.Atoi(rest[loc[2]:loc[3]]); err == nil {
			m.Facility = pri / 8
			m.Severity = pri % 8
		}
		rest = rest[loc[1]:]
	}

	if rfc5424VersionRe.MatchString(rest) {
		parseRFC5424(&m, rest[2:], receivedAt)
		return m
	}

	parseRFC3164(&m, rest, receivedAt)
	return m
}

// parseRFC3164 handles legacy BSD-style syslog (RFC 3164), MikroTik
// RouterOS's default format (BSD framing) and Cisco IOS (BSD framing plus
// an optional leading sequence number and a "%FACILITY-SEVERITY-MNEMONIC:"
// marker in place of a plain tag).
func parseRFC3164(m *Message, rest string, receivedAt time.Time) {
	rest = ciscoSeqRe.ReplaceAllString(rest, "")

	// Best-effort RFC 3164 timestamp ("Mon Jan 02 15:04:05 hostname ..."),
	// also covers Cisco's "*Sep 28 12:00:00.123:" once seconds/millis are
	// tolerated. The timestamp carries no year (RFC 3164 doesn't include
	// one), so we anchor it to receivedAt's year -- if that lands in the
	// future (e.g. a message timestamped Dec 31 arrives just after a Jan 1
	// rollover), roll back a year.
	if loc := bsdTimestampRe.FindStringSubmatch(rest); loc != nil {
		month, day, clock, hostname, tail := loc[1], loc[2], loc[3], loc[4], loc[5]
		layout := "2006 Jan 2 15:04:05"
		ts, err := time.ParseInLocation(layout, receivedAt.Format("2006")+" "+month+" "+day+" "+clock, receivedAt.Location())
		if err == nil {
			if ts.After(receivedAt.Add(24 * time.Hour)) {
				ts = ts.AddDate(-1, 0, 0)
			}
			m.DeviceTimestamp = &ts
			m.TimestampEstimated = false
		}
		m.Hostname = hostname
		rest = tail
	}

	if loc := ciscoMnemonicRe.FindStringSubmatchIndex(rest); loc != nil {
		m.Tag = rest[loc[2]:loc[3]]
		rest = rest[loc[1]:]
	} else if loc := tagRe.FindStringSubmatchIndex(rest); loc != nil {
		m.Tag = rest[loc[2]:loc[3]]
		rest = rest[loc[1]:]
	}
	// If nothing meaningful survives the trim, leave m.Body as the full
	// original line (already set by Parse before this function was
	// called) rather than overwriting it with an empty string.
	if trimmed := strings.TrimSpace(rest); trimmed != "" {
		m.Body = trimmed
	}
}

// parseRFC5424 handles the structured RFC 5424 header:
// TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
// (rest is everything after "<PRI>1 "). STRUCTURED-DATA is the one field
// that can itself contain spaces (inside its own [...] elements), so it's
// consumed with a small bracket-aware scanner rather than a plain split.
func parseRFC5424(m *Message, rest string, receivedAt time.Time) {
	next := func(s string) (token, remainder string) {
		s = strings.TrimLeft(s, " ")
		i := strings.IndexByte(s, ' ')
		if i < 0 {
			return s, ""
		}
		return s[:i], s[i+1:]
	}

	var timestampTok, hostnameTok, appNameTok string
	timestampTok, rest = next(rest)
	hostnameTok, rest = next(rest)
	appNameTok, rest = next(rest)
	_, rest = next(rest) // PROCID -- not currently surfaced
	_, rest = next(rest) // MSGID -- not currently surfaced

	// STRUCTURED-DATA: either "-" (nil) or one or more "[...]" elements
	// back to back with no separating space.
	rest = strings.TrimLeft(rest, " ")
	if strings.HasPrefix(rest, "[") {
		depth := 0
		i := 0
		for i < len(rest) {
			switch rest[i] {
			case '[':
				depth++
			case ']':
				depth--
			}
			i++
			if depth == 0 {
				break
			}
		}
		rest = strings.TrimLeft(rest[i:], " ")
	} else if strings.HasPrefix(rest, "- ") {
		rest = rest[2:]
	} else if rest == "-" {
		rest = ""
	}

	if timestampTok != "-" {
		if ts, err := time.Parse(time.RFC3339Nano, timestampTok); err == nil {
			m.DeviceTimestamp = &ts
			m.TimestampEstimated = false
		}
	}
	if hostnameTok != "-" {
		m.Hostname = hostnameTok
	}
	if appNameTok != "-" {
		m.Tag = appNameTok
	}

	// A leading UTF-8 BOM (EF BB BF) on the MSG field is explicitly allowed
	// by RFC 5424 section 6.4 and isn't part of the message text.
	rest = strings.TrimPrefix(rest, "﻿")
	// If nothing survives (MSG was empty/whitespace-only), leave m.Body as
	// the full original line (already set by Parse) rather than storing
	// blank. A literal "-" (RFC 5424's nil-MSG marker) is kept as-is.
	if trimmed := strings.TrimSpace(rest); trimmed != "" {
		m.Body = trimmed
	}
}

// deviceCache is a periodically-refreshed, in-memory address->deviceID
// lookup so resolving "unknown device vs. known device" doesn't cost a
// database round-trip per incoming message (syslog can arrive at a much
// higher rate than the other pollers in this codebase).
type deviceCache struct {
	mu   sync.RWMutex
	byIP map[string]int64
}

func newDeviceCache() *deviceCache {
	return &deviceCache{byIP: map[string]int64{}}
}

func (c *deviceCache) lookup(ip string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	id, ok := c.byIP[ip]
	return id, ok
}

func (c *deviceCache) refresh(ctx context.Context, db *pgxpool.Pool) {
	rows, err := db.Query(ctx, `SELECT id, address FROM devices WHERE address <> ''`)
	if err != nil {
		log.Printf("syslog: device cache refresh: %v", err)
		return
	}
	defer rows.Close()
	next := map[string]int64{}
	for rows.Next() {
		var id int64
		var addr string
		if err := rows.Scan(&id, &addr); err != nil {
			continue
		}
		next[addr] = id
	}
	if err := rows.Err(); err != nil {
		log.Printf("syslog: device cache refresh: %v", err)
		return
	}
	c.mu.Lock()
	c.byIP = next
	c.mu.Unlock()
}

func (c *deviceCache) runPeriodic(ctx context.Context, db *pgxpool.Pool, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	c.refresh(ctx, db)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx, db)
		}
	}
}

// rateLimiter is a simple per-source token bucket so one noisy/misconfigured
// device can't fill the database or starve other devices' messages out of
// the queue. Buckets for sources that go quiet are never explicitly
// evicted; the fleet size this is designed for (hundreds, not millions, of
// distinct source IPs) makes that an acceptable tradeoff against the
// complexity of an eviction policy.
type rateLimiter struct {
	mu      sync.Mutex
	perSec  float64
	burst   float64
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	updated  time.Time
}

func newRateLimiter(perSec, burst float64) *rateLimiter {
	if perSec <= 0 {
		perSec = 50
	}
	if burst <= 0 {
		burst = perSec * 2
	}
	return &rateLimiter{perSec: perSec, burst: burst, buckets: map[string]*bucket{}}
}

// allow reports whether a message from ip may proceed right now, consuming
// one token if so.
func (r *rateLimiter) allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	b := r.buckets[ip]
	if b == nil {
		b = &bucket{tokens: r.burst, updated: now}
		r.buckets[ip] = b
	}
	elapsed := now.Sub(b.updated).Seconds()
	b.updated = now
	b.tokens += elapsed * r.perSec
	if b.tokens > r.burst {
		b.tokens = r.burst
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Config controls the receiver's resource limits. All fields have sane
// zero-value defaults applied in ListenAndServe/NewConfig, so a caller can
// pass a partially-populated Config.
type Config struct {
	// QueueSize bounds the channel between the socket readers and the DB
	// writer workers. When full, new messages are dropped (never blocking
	// the socket read loop) and counted.
	QueueSize int
	// QueueWorkers is how many goroutines drain the queue into the
	// database concurrently.
	QueueWorkers int
	// RatePerSecond / RateBurst configure the per-source-IP token bucket.
	RatePerSecond float64
	RateBurst     float64
	// DeviceCacheRefresh controls how often the address->device lookup
	// table is refreshed from the devices table.
	DeviceCacheRefresh time.Duration
}

func (c Config) withDefaults() Config {
	if c.QueueSize <= 0 {
		c.QueueSize = 5000
	}
	if c.QueueWorkers <= 0 {
		c.QueueWorkers = 4
	}
	if c.RatePerSecond <= 0 {
		c.RatePerSecond = 50
	}
	if c.RateBurst <= 0 {
		c.RateBurst = c.RatePerSecond * 4
	}
	if c.DeviceCacheRefresh <= 0 {
		c.DeviceCacheRefresh = 30 * time.Second
	}
	return c
}

// queuedMessage pairs a parsed Message with the receive time it should fall
// back to when the message itself carried no usable timestamp.
type queuedMessage struct {
	msg        Message
	receivedAt time.Time
}

// receiver owns the shared state (queue, rate limiter, device cache, drop
// counters) between the UDP/TCP accept loops and the DB writer workers.
type receiver struct {
	db      *pgxpool.Pool
	queue   chan queuedMessage
	limiter *rateLimiter
	devices *deviceCache

	droppedRateLimited int64
	droppedQueueFull   int64
	mu                 sync.Mutex
	lastDropLog        time.Time
}

func (rc *receiver) ingest(sourceIP string, raw string) {
	if !rc.limiter.allow(sourceIP) {
		rc.mu.Lock()
		rc.droppedRateLimited++
		rc.maybeLogDrops()
		rc.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	msg := Parse(sourceIP, raw, now)
	select {
	case rc.queue <- queuedMessage{msg: msg, receivedAt: now}:
	default:
		rc.mu.Lock()
		rc.droppedQueueFull++
		rc.maybeLogDrops()
		rc.mu.Unlock()
	}
}

// maybeLogDrops logs a summary at most once every 30s, rather than once per
// dropped message, so a sustained flood doesn't itself become a logging
// flood. Caller must hold rc.mu.
func (rc *receiver) maybeLogDrops() {
	if time.Since(rc.lastDropLog) < 30*time.Second {
		return
	}
	rc.lastDropLog = time.Now()
	log.Printf("syslog: dropped %d message(s) (rate-limited) and %d message(s) (queue full) in the last ~30s",
		rc.droppedRateLimited, rc.droppedQueueFull)
	rc.droppedRateLimited = 0
	rc.droppedQueueFull = 0
}

func (rc *receiver) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case qm := <-rc.queue:
			deviceID, known := rc.devices.lookup(qm.msg.SourceIP)
			storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			if err := store(storeCtx, rc.db, qm.msg, qm.receivedAt, deviceID, known); err != nil {
				log.Printf("syslog: store failed (%s): %v", qm.msg.SourceIP, err)
			}
			cancel()
		}
	}
}

// store persists a parsed message. deviceID/known come from the receiver's
// device cache; known=false stores a NULL device_id ("unknown device") but
// the message itself is always kept, never dropped for being unmatched.
func store(ctx context.Context, db *pgxpool.Pool, m Message, receivedAt time.Time, deviceID int64, known bool) error {
	messageTS := receivedAt
	if m.DeviceTimestamp != nil {
		messageTS = *m.DeviceTimestamp
	}
	var deviceIDArg any
	if known {
		deviceIDArg = deviceID
	}
	_, err := db.Exec(ctx, `INSERT INTO syslog_messages
			(received_at,source_ip,facility,severity,hostname,tag,message,device_id,message_timestamp,timestamp_estimated)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		receivedAt, m.SourceIP, nullableInt(m.Facility), nullableInt(m.Severity), nullableStr(m.Hostname), nullableStr(m.Tag), m.Body,
		deviceIDArg, messageTS, m.TimestampEstimated)
	return err
}

func nullableInt(v int) any {
	if v < 0 {
		return nil
	}
	return v
}
func nullableStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// ListenAndServe runs both a UDP and a TCP syslog listener on addr (e.g.
// ":514" -- the standard syslog port; see deployments/ubuntu-24.04/
// routingnms-api.service, which grants CAP_NET_BIND_SERVICE so the service
// can bind it without running as root) until ctx is cancelled. Incoming
// lines are rate-limited per source, queued through a bounded channel, and
// written to the database by a small worker pool -- a slow database can
// make the queue back up and start dropping the newest messages, but it can
// never block the socket read loop itself. Storage errors are logged and
// otherwise ignored so one bad row never brings down ingestion for the rest
// of the fleet.
func ListenAndServe(ctx context.Context, db *pgxpool.Pool, addr string, cfg Config) error {
	cfg = cfg.withDefaults()

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	tcpLn, err := net.Listen("tcp", addr)
	if err != nil {
		udpConn.Close()
		return err
	}

	rc := &receiver{
		db:      db,
		queue:   make(chan queuedMessage, cfg.QueueSize),
		limiter: newRateLimiter(cfg.RatePerSecond, cfg.RateBurst),
		devices: newDeviceCache(),
	}
	go rc.devices.runPeriodic(ctx, db, cfg.DeviceCacheRefresh)
	for i := 0; i < cfg.QueueWorkers; i++ {
		go rc.worker(ctx)
	}

	go func() {
		<-ctx.Done()
		udpConn.Close()
		tcpLn.Close()
	}()

	go serveUDP(ctx, rc, udpConn)
	go serveTCP(ctx, rc, tcpLn)

	log.Printf("syslog receiver listening on %s (udp+tcp), queue=%d workers=%d rate=%.0f/s burst=%.0f",
		addr, cfg.QueueSize, cfg.QueueWorkers, cfg.RatePerSecond, cfg.RateBurst)
	<-ctx.Done()
	return nil
}

func serveUDP(ctx context.Context, rc *receiver, conn *net.UDPConn) {
	// A UDP datagram larger than this buffer is truncated by ReadFromUDP
	// itself (excess bytes are discarded by the kernel/runtime, not left
	// for a subsequent read), which already caps a single message's size
	// independent of the MaxMessageBytes truncation Parse also applies.
	buf := make([]byte, MaxMessageBytes+256)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		line := strings.TrimRight(string(buf[:n]), "\r\n")
		if line == "" {
			continue
		}
		rc.ingest(remote.IP.String(), line)
	}
}

func serveTCP(ctx context.Context, rc *receiver, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go handleTCPConn(ctx, rc, conn)
	}
}

func handleTCPConn(ctx context.Context, rc *receiver, conn net.Conn) {
	defer conn.Close()
	remoteHost, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 4096), MaxMessageBytes+256)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		rc.ingest(remoteHost, line)
	}
}

// PruneOlderThan deletes syslog rows older than the given age. Intended to
// be called periodically (see cmd/api) so an unattended NMS doesn't fill its
// disk with syslog history indefinitely.
func PruneOlderThan(ctx context.Context, db *pgxpool.Pool, age time.Duration) (int64, error) {
	tag, err := db.Exec(ctx, `DELETE FROM syslog_messages WHERE received_at < $1`, time.Now().Add(-age))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
