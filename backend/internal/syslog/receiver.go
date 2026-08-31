// Package syslog implements a minimal syslog receiver (RFC 3164 / loosely
// RFC 5424 tolerant) so OLTs, routers, switches and CMTS gear in the fleet
// can point their "syslog server" setting at this NMS. It intentionally does
// not aim for full RFC 5424 structured-data parsing -- ISP access-layer
// gear overwhelmingly emits legacy BSD-style syslog, and a best-effort parse
// that still stores the raw line is more useful than a strict parser that
// drops anything it doesn't fully understand.
package syslog

import (
	"bufio"
	"context"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// priRe matches the leading "<PRI>" of a syslog message, e.g. "<134>".
// PRI = facility*8 + severity, per RFC 3164 section 4.1.1.
var priRe = regexp.MustCompile(`^<(\d{1,3})>`)

// tagRe pulls a best-effort "tag" (process/program name) off the front of
// the message body, e.g. "sshd[1234]: " or "OLT_ALARM: ".
var tagRe = regexp.MustCompile(`^(\S+?):\s?`)

type Message struct {
	SourceIP string
	Facility int
	Severity int
	Hostname string
	Tag      string
	Body     string
}

// Parse extracts PRI (facility/severity) and a best-effort tag from a raw
// syslog line. Anything it can't confidently parse still comes back with
// the full original text in Body, so nothing is silently dropped.
func Parse(sourceIP string, raw string) Message {
	m := Message{SourceIP: sourceIP, Facility: -1, Severity: -1, Body: raw}
	rest := raw
	if loc := priRe.FindStringSubmatchIndex(rest); loc != nil {
		if pri, err := strconv.Atoi(rest[loc[2]:loc[3]]); err == nil {
			m.Facility = pri / 8
			m.Severity = pri % 8
		}
		rest = rest[loc[1]:]
	}
	// Best-effort RFC 3164 timestamp skip ("Mon Jan 02 15:04:05 ") -- if the
	// next token doesn't parse as a hostname-looking word we just leave rest
	// alone; we don't need the device's own clock, we have received_at.
	fields := strings.SplitN(rest, " ", 5)
	if len(fields) >= 4 && looksLikeMonth(fields[0]) {
		m.Hostname = fields[3]
		if len(fields) == 5 {
			rest = fields[4]
		} else {
			rest = ""
		}
	}
	if loc := tagRe.FindStringSubmatchIndex(rest); loc != nil {
		m.Tag = rest[loc[2]:loc[3]]
		rest = rest[loc[1]:]
	}
	m.Body = strings.TrimSpace(rest)
	if m.Body == "" {
		m.Body = raw
	}
	return m
}

func looksLikeMonth(s string) bool {
	switch s {
	case "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec":
		return true
	default:
		return false
	}
}

// Store persists a parsed message. Kept as a thin standalone function
// (rather than a method with lots of state) so the UDP and TCP listeners
// below can share it without coupling to a larger receiver struct.
func store(ctx context.Context, db *pgxpool.Pool, m Message) error {
	_, err := db.Exec(ctx, `INSERT INTO syslog_messages (source_ip,facility,severity,hostname,tag,message) VALUES ($1,$2,$3,$4,$5,$6)`,
		m.SourceIP, nullableInt(m.Facility), nullableInt(m.Severity), nullableStr(m.Hostname), nullableStr(m.Tag), m.Body)
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
// ":514" -- the standard syslog port, or ":1514" when running unprivileged)
// until ctx is cancelled. Each accepted line is parsed and stored; storage
// errors are logged and otherwise ignored so one bad row never brings down
// ingestion for the rest of the fleet.
func ListenAndServe(ctx context.Context, db *pgxpool.Pool, addr string) error {
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

	go func() {
		<-ctx.Done()
		udpConn.Close()
		tcpLn.Close()
	}()

	go serveUDP(ctx, db, udpConn)
	go serveTCP(ctx, db, tcpLn)

	log.Printf("syslog receiver listening on %s (udp+tcp)", addr)
	<-ctx.Done()
	return nil
}

func serveUDP(ctx context.Context, db *pgxpool.Pool, conn *net.UDPConn) {
	buf := make([]byte, 8192)
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
		msg := Parse(remote.IP.String(), line)
		storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if err := store(storeCtx, db, msg); err != nil {
			log.Printf("syslog: store failed (udp, %s): %v", remote.IP, err)
		}
		cancel()
	}
}

func serveTCP(ctx context.Context, db *pgxpool.Pool, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		go handleTCPConn(ctx, db, conn)
	}
}

func handleTCPConn(ctx context.Context, db *pgxpool.Pool, conn net.Conn) {
	defer conn.Close()
	remoteHost, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 8192), 65536)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		msg := Parse(remoteHost, line)
		storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if err := store(storeCtx, db, msg); err != nil {
			log.Printf("syslog: store failed (tcp, %s): %v", remoteHost, err)
		}
		cancel()
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
