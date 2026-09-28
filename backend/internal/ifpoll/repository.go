package ifpoll

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// EnabledDevice is the subset of devices the interface poller iterates --
// mirrors ping.IcmpEnabledDevice / portcheck.EnabledDevice, but selects the
// same SNMP credential columns devices.DiscoveryTarget already reads (just
// batched into one query instead of one row at a time).
type EnabledDevice struct {
	ID              string
	Address         string
	SNMPPort        uint16
	Credentials     snmp.Credentials
	Timeout         time.Duration
	IntervalSeconds int
}

// PriorPort is the previously-stored state for one port, used both to detect
// an up/down transition and to compute the traffic rate since the last poll.
type PriorPort struct {
	AdminUp      bool
	OperUp       bool
	InOctets     uint64
	OutOctets    uint64
	CounterWidth int
	SampledAt    time.Time
}

// Repository persists interface-poll results and reads the set of
// SNMP-enabled devices.
type Repository struct {
	DB *pgxpool.Pool
}

// ListEnabled returns every enabled device with SNMP monitoring turned on.
func (r Repository) ListEnabled(ctx context.Context) ([]EnabledDevice, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("ifpoll repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,address,COALESCE(snmp_port,161),snmp_version,COALESCE(snmp_community,''),COALESCE(snmp_username,''),COALESCE(snmp_auth_protocol,''),COALESCE(snmp_auth_password,''),COALESCE(snmp_priv_protocol,''),COALESCE(snmp_priv_password,''),COALESCE(snmp_timeout_ms,3000),COALESCE(if_poll_interval_seconds,60)
		FROM devices WHERE enabled=true AND snmp_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnabledDevice{}
	for rows.Next() {
		var d EnabledDevice
		var version string
		var timeoutMS int
		if err := rows.Scan(&d.ID, &d.Address, &d.SNMPPort, &version, &d.Credentials.Community, &d.Credentials.Username, &d.Credentials.AuthProto, &d.Credentials.AuthPass, &d.Credentials.PrivProto, &d.Credentials.PrivPass, &timeoutMS, &d.IntervalSeconds); err != nil {
			return nil, err
		}
		d.Credentials.Version = snmp.Version(version)
		d.Timeout = time.Duration(timeoutMS) * time.Millisecond
		if d.IntervalSeconds <= 0 {
			d.IntervalSeconds = 60
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// PriorState returns the previously-stored state for every known port of
// deviceID, keyed by if_index, so a poll can detect transitions and compute
// rates against the last sample.
func (r Repository) PriorState(ctx context.Context, deviceID string) (map[int64]PriorPort, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("ifpoll repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT if_index,admin_up,oper_up,in_octets,out_octets,counter_width,counter_sampled_at FROM interfaces WHERE device_id=$1`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]PriorPort{}
	for rows.Next() {
		var idx int64
		var p PriorPort
		var sampledAt *time.Time
		if err := rows.Scan(&idx, &p.AdminUp, &p.OperUp, &p.InOctets, &p.OutOctets, &p.CounterWidth, &sampledAt); err != nil {
			return nil, err
		}
		if sampledAt != nil {
			p.SampledAt = *sampledAt
		}
		out[idx] = p
	}
	return out, rows.Err()
}

// SavePoll upserts current per-port state for deviceID and inserts an
// interface_transitions row only for ports whose admin/oper state actually
// changed since the prior poll -- current state is written every cycle (the
// octet counters/rates need to stay fresh), but transition history stays
// sparse.
func (r Repository) SavePoll(ctx context.Context, deviceID string, ports []PortState, prior map[int64]PriorPort, now time.Time) error {
	if r.DB == nil {
		return fmt.Errorf("ifpoll repository is not initialized")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, p := range ports {
		prev, hadPrior := prior[p.IfIndex]

		var inRate, outRate *float64
		if hadPrior && prev.CounterWidth == p.CounterWidth {
			if v, ok := computeRate(prev.InOctets, prev.SampledAt, p.InOctets, now, p.CounterWidth); ok {
				inRate = &v
			}
			if v, ok := computeRate(prev.OutOctets, prev.SampledAt, p.OutOctets, now, p.CounterWidth); ok {
				outRate = &v
			}
		}

		transitioned := !hadPrior || prev.AdminUp != p.AdminUp || prev.OperUp != p.OperUp

		var lastTransitionAt *time.Time
		if transitioned {
			t := now
			lastTransitionAt = &t
		}

		_, err := tx.Exec(ctx, `INSERT INTO interfaces
				(device_id,if_index,name,description,admin_up,oper_up,in_octets,out_octets,in_errors,out_errors,
				 if_speed_bps,counter_width,in_rate_bps,out_rate_bps,counter_sampled_at,last_transition_at,last_discovered_at)
			VALUES ($1,$2,$3,$3,$4,$5,$6,$7,0,0,$8,$9,$10,$11,$12,COALESCE($13,(SELECT last_transition_at FROM interfaces WHERE device_id=$1 AND if_index=$2)),NOW())
			ON CONFLICT (device_id,if_index) DO UPDATE SET
				name=EXCLUDED.name, description=EXCLUDED.description,
				admin_up=EXCLUDED.admin_up, oper_up=EXCLUDED.oper_up,
				in_octets=EXCLUDED.in_octets, out_octets=EXCLUDED.out_octets,
				if_speed_bps=EXCLUDED.if_speed_bps, counter_width=EXCLUDED.counter_width,
				in_rate_bps=EXCLUDED.in_rate_bps, out_rate_bps=EXCLUDED.out_rate_bps,
				counter_sampled_at=EXCLUDED.counter_sampled_at,
				last_transition_at=COALESCE(EXCLUDED.last_transition_at, interfaces.last_transition_at),
				last_discovered_at=NOW()`,
			deviceID, p.IfIndex, p.Description, p.AdminUp, p.OperUp, p.InOctets, p.OutOctets,
			p.SpeedBps, p.CounterWidth, inRate, outRate, now, lastTransitionAt)
		if err != nil {
			return fmt.Errorf("save interface %d: %w", p.IfIndex, err)
		}

		if transitioned {
			if _, err := tx.Exec(ctx, `INSERT INTO interface_transitions (device_id,if_index,oper_up,admin_up,changed_at) VALUES ($1,$2,$3,$4,$5)`,
				deviceID, p.IfIndex, p.OperUp, p.AdminUp, now); err != nil {
				return fmt.Errorf("record transition for %d: %w", p.IfIndex, err)
			}
		}
	}

	if _, err := tx.Exec(ctx, `INSERT INTO device_snmp_poll_state (device_id,reachable,last_attempt_at,last_success_at,last_error)
		VALUES ($1,true,$2,$2,'')
		ON CONFLICT (device_id) DO UPDATE SET reachable=true, last_attempt_at=EXCLUDED.last_attempt_at, last_success_at=EXCLUDED.last_success_at, last_error=''`,
		deviceID, now); err != nil {
		return fmt.Errorf("mark reachable: %w", err)
	}

	return tx.Commit(ctx)
}

// MarkUnreachable records an SNMP poll failure for deviceID. It intentionally
// touches only device_snmp_poll_state, never the interfaces table -- ports
// keep their last-known state so the read API can report them as stale
// rather than forcing them to "down". Unknown is not the same as down.
func (r Repository) MarkUnreachable(ctx context.Context, deviceID string, errMsg string, now time.Time) error {
	if r.DB == nil {
		return fmt.Errorf("ifpoll repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `INSERT INTO device_snmp_poll_state (device_id,reachable,last_attempt_at,last_error)
		VALUES ($1,false,$2,$3)
		ON CONFLICT (device_id) DO UPDATE SET reachable=false, last_attempt_at=EXCLUDED.last_attempt_at, last_error=EXCLUDED.last_error`,
		deviceID, now, errMsg)
	return err
}
