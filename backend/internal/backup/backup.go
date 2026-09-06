// Package backup implements RoutingNMS's configuration backup/restore,
// adapted from Uptime Kuma's "Backup" feature (server/server.js's
// `uploadBackup` handler and its matching export). Kuma's backup is a JSON
// export/import of monitor CONFIGURATION only (notifications, proxies,
// monitors + tag assignments) -- never heartbeat/metric history, since that
// would make backups huge and isn't configuration. This package matches
// that scope for RoutingNMS's own data model:
//
//   - devices (internal/devices)             -- Kuma's "monitors"
//   - tags + device/tag assignments          (internal/tags)
//   - device groups + memberships            (internal/devicegroups) --
//     RoutingNMS-specific, Kuma has no equivalent
//   - notification channels                  (internal/alerts) -- Kuma's
//     "notifications"
//   - alert rules                            (internal/alerts) -- Kuma's
//     rule engine is per-monitor thresholds; RoutingNMS's is a separate
//     named-rule table, exported/imported alongside channels
//
// Deliberately NOT included: heartbeats, metric_samples, incidents, or any
// other historical/time-series data -- matching Kuma's own scope.
//
// Security: a device's SNMP credentials (community string, v3 auth/priv
// passwords) are never present in the bundle. devices.Record -- the type
// devices are exported as -- simply doesn't carry those fields (only
// snmpEnabled/snmpVersion/snmpPort, which aren't secret), the same
// deliberate omission the existing "Clone device" feature in the frontend
// relies on (see cloneDevice in frontend/app/(noc)/devices/page.tsx, which
// pre-fills a new-device form from an existing one but never carries SNMP
// credentials across). A restored device therefore always comes back with
// SNMP monitoring disabled; the operator must re-enter credentials and
// re-enable it by hand after import. This is intentional, not an
// oversight -- a config backup file is something people email around,
// commit to a wiki, or leave in a downloads folder, and it must never be a
// bundle of every device's SNMP secrets.
package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/alerts"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devicegroups"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/tags"
)

// Version is the current bundle format version, written into every export
// and checked (loosely -- unrecognized versions are still attempted) on
// import.
const Version = "1.0"

// Import modes, mirroring Kuma's three `importHandle` options exactly:
//
//   - Overwrite: delete all existing devices/tags/device groups/
//     notification channels/alert rules for the target tenant (alert rules
//     are instance-wide, see Repository.DeleteAllRules), then import
//     everything in the bundle fresh.
//   - Keep: import everything in the bundle as new rows, alongside existing
//     data. No dedup by name -- except where RoutingNMS's schema enforces a
//     uniqueness constraint Kuma's doesn't have (see Import's doc comment).
//   - Skip: for each entity, skip importing a row whose name already exists
//     in the target; only rows with new names are imported.
const (
	ModeOverwrite = "overwrite"
	ModeKeep      = "keep"
	ModeSkip      = "skip"
)

// Bundle is the full exported configuration snapshot. Every field is
// exported as JSON so a bundle round-trips as plain, readable JSON -- an
// operator can open one in a text editor.
type Bundle struct {
	Version    string    `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
	TenantID   string    `json:"tenantId"`

	Devices              []devices.Record          `json:"devices"`
	DeviceTagAssignments []tags.Assignment         `json:"deviceTagAssignments"`
	Tags                 []tags.Tag                `json:"tags"`
	DeviceGroups         []devicegroups.Group      `json:"deviceGroups"`
	DeviceGroupMembers   []devicegroups.Member     `json:"deviceGroupMembers"`
	NotificationChannels []alerts.PersistedChannel `json:"notificationChannels"`
	AlertRules           []alerts.PersistedRule    `json:"alertRules"`
}

// Repositories bundles the existing repository types this package reads
// from and writes to -- Export/Import take one of these rather than a raw
// *pgxpool.Pool, so they reuse each package's own query/validation logic
// instead of duplicating SQL.
type Repositories struct {
	Devices      devices.Repository
	Tags         tags.Repository
	DeviceGroups devicegroups.Repository
	Alerts       alerts.Repository
}

// Result summarizes what Import actually did -- returned to the caller (and
// surfaced in the UI) since "skip" and "keep" modes silently rename/skip
// some rows rather than failing outright.
type Result struct {
	Mode                        string   `json:"mode"`
	DevicesImported             int      `json:"devicesImported"`
	DevicesSkipped              int      `json:"devicesSkipped"`
	TagsImported                int      `json:"tagsImported"`
	DeviceGroupsImported        int      `json:"deviceGroupsImported"`
	NotificationChannelsAdded   int      `json:"notificationChannelsImported"`
	NotificationChannelsSkipped int      `json:"notificationChannelsSkipped"`
	AlertRulesImported          int      `json:"alertRulesImported"`
	AlertRulesSkipped           int      `json:"alertRulesSkipped"`
	Warnings                    []string `json:"warnings,omitempty"`
}

// Export assembles a Bundle for one tenant. tenantID scopes devices (as
// devices.Record.OrganizationID), tags, device groups and notification
// channels; alert_rules has no tenant_id column in RoutingNMS's schema (it
// predates per-tenant scoping) so every alert rule is included regardless of
// tenantID -- a documented limitation on a multi-tenant deployment.
func Export(ctx context.Context, repos Repositories, tenantID string) (*Bundle, error) {
	deviceList, err := repos.Devices.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("loading devices: %w", err)
	}
	deviceIDs := make(map[string]bool, len(deviceList))
	for _, d := range deviceList {
		deviceIDs[d.ID] = true
	}

	tagList, err := repos.Tags.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("loading tags: %w", err)
	}

	allAssignments, err := repos.Tags.AllAssignments(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading tag assignments: %w", err)
	}
	assignments := []tags.Assignment{}
	for _, a := range allAssignments {
		if a.SubjectType == "device" && deviceIDs[a.SubjectID] {
			assignments = append(assignments, a)
		}
	}

	groupList, err := repos.DeviceGroups.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("loading device groups: %w", err)
	}
	groupIDs := make(map[int64]bool, len(groupList))
	for _, g := range groupList {
		groupIDs[g.ID] = true
	}

	allMembers, err := repos.DeviceGroups.AllMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading device group members: %w", err)
	}
	members := []devicegroups.Member{}
	for _, m := range allMembers {
		if m.SubjectType == "device" && groupIDs[m.GroupID] && deviceIDs[m.SubjectID] {
			members = append(members, m)
		}
	}

	channels, err := repos.Alerts.ListChannels(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("loading notification channels: %w", err)
	}

	rules, err := repos.Alerts.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading alert rules: %w", err)
	}

	return &Bundle{
		Version:              Version,
		ExportedAt:           time.Now().UTC(),
		TenantID:             tenantID,
		Devices:              deviceList,
		DeviceTagAssignments: assignments,
		Tags:                 tagList,
		DeviceGroups:         groupList,
		DeviceGroupMembers:   members,
		NotificationChannels: channels,
		AlertRules:           rules,
	}, nil
}

// Import loads a Bundle into the target tenant (tenantID -- the destination
// tenant, which need not match bundle.TenantID: restoring a bundle into a
// different tenant than it was exported from is intentional and supported).
//
// Two deviations from Kuma's exact semantics, both forced by schema
// constraints Kuma's own tables don't have:
//
//   - tags and device_groups each have a UNIQUE(tenant_id,name) constraint,
//     so "keep" mode cannot insert a same-named duplicate the way Kuma's
//     schema-less monitor tags would. Both are resolved by name (get the
//     existing tenant/name row, or create it) in every mode, functionally
//     equivalent to "skip" for those two entities specifically -- there is
//     no meaningful "keep two tags/groups with the same name" to offer.
//   - devices has a UNIQUE(organization_id,name) constraint, so "keep" mode
//     importing a device whose name collides with an existing one appends
//     " (import <n>)" to the name instead of failing outright.
//
// Every device is imported with SNMP monitoring disabled and no
// credentials, regardless of its exported snmpEnabled/snmpVersion -- see
// this package's doc comment.
func Import(ctx context.Context, repos Repositories, tenantID string, bundle *Bundle, mode string) (*Result, error) {
	if bundle == nil {
		return nil, fmt.Errorf("bundle is required")
	}
	switch mode {
	case ModeOverwrite, ModeKeep, ModeSkip:
	default:
		return nil, fmt.Errorf("mode must be %q, %q or %q", ModeOverwrite, ModeKeep, ModeSkip)
	}

	result := &Result{Mode: mode}

	if mode == ModeOverwrite {
		// Order matters: devices before tags/groups so the manual
		// tag_assignments/device_group_members cleanup in
		// DeleteAllForOrg still finds the (about-to-be-deleted)
		// device ids it needs to clean up after.
		if err := repos.Devices.DeleteAllForOrg(ctx, tenantID); err != nil {
			return nil, fmt.Errorf("clearing existing devices: %w", err)
		}
		if err := repos.Tags.DeleteAllForTenant(ctx, tenantID); err != nil {
			return nil, fmt.Errorf("clearing existing tags: %w", err)
		}
		if err := repos.DeviceGroups.DeleteAllForTenant(ctx, tenantID); err != nil {
			return nil, fmt.Errorf("clearing existing device groups: %w", err)
		}
		if err := repos.Alerts.DeleteAllChannelsForTenant(ctx, tenantID); err != nil {
			return nil, fmt.Errorf("clearing existing notification channels: %w", err)
		}
		if err := repos.Alerts.DeleteAllRules(ctx); err != nil {
			return nil, fmt.Errorf("clearing existing alert rules: %w", err)
		}
		result.Warnings = append(result.Warnings, "overwrite mode cleared ALL alert rules instance-wide (alert_rules has no per-tenant scoping in this schema), not just this tenant's")
	}

	// Tags and device groups first (by name) so devices/memberships below
	// can look up their new ids.
	tagByOldID := map[int64]tags.Tag{}
	for _, t := range bundle.Tags {
		created, err := repos.Tags.GetOrCreateByName(ctx, tenantID, t.Name, t.Color)
		if err != nil {
			return nil, fmt.Errorf("importing tag %q: %w", t.Name, err)
		}
		tagByOldID[t.ID] = created
		result.TagsImported++
	}

	groupByOldID := map[int64]devicegroups.Group{}
	for _, g := range bundle.DeviceGroups {
		created, err := repos.DeviceGroups.GetOrCreateByName(ctx, tenantID, g.Name, g.SortOrder)
		if err != nil {
			return nil, fmt.Errorf("importing device group %q: %w", g.Name, err)
		}
		groupByOldID[g.ID] = created
		result.DeviceGroupsImported++
	}

	// Notification channels, before alert rules (rules reference channel
	// ids in notification_channel_ids -- see channelIDMap below).
	channelByOldID := map[int64]int64{}
	for _, ch := range bundle.NotificationChannels {
		if mode == ModeSkip {
			exists, err := repos.Alerts.ChannelExistsByName(ctx, tenantID, ch.Name)
			if err != nil {
				return nil, fmt.Errorf("checking notification channel %q: %w", ch.Name, err)
			}
			if exists {
				result.NotificationChannelsSkipped++
				continue
			}
		}
		toCreate := ch
		toCreate.ID = 0
		toCreate.TenantID = tenantID
		created, err := repos.Alerts.SaveChannel(ctx, toCreate)
		if err != nil {
			return nil, fmt.Errorf("importing notification channel %q: %w", ch.Name, err)
		}
		channelByOldID[ch.ID] = created.ID
		result.NotificationChannelsAdded++
	}

	// Devices.
	deviceByOldID := map[string]string{}
	for _, d := range bundle.Devices {
		if mode == ModeSkip {
			exists, err := repos.Devices.ExistsByName(ctx, tenantID, d.Name)
			if err != nil {
				return nil, fmt.Errorf("checking device %q: %w", d.Name, err)
			}
			if exists {
				result.DevicesSkipped++
				continue
			}
		}
		name := d.Name
		if mode == ModeKeep {
			name = uniqueDeviceName(ctx, repos.Devices, tenantID, name)
		}
		created, err := importDevice(ctx, repos.Devices, tenantID, name, d)
		if err != nil {
			return nil, fmt.Errorf("importing device %q: %w", d.Name, err)
		}
		deviceByOldID[d.ID] = created.ID
		result.DevicesImported++
	}

	// Device group memberships and tag assignments, using the id maps built
	// above -- only for devices that were actually imported (skipped
	// devices leave no mapping, so their memberships/assignments are
	// dropped rather than pointing at a device that doesn't exist here).
	for _, m := range bundle.DeviceGroupMembers {
		newDeviceID, ok := deviceByOldID[m.SubjectID]
		if !ok {
			continue
		}
		newGroup, ok := groupByOldID[m.GroupID]
		if !ok {
			continue
		}
		if err := repos.DeviceGroups.SetForSubject(ctx, "device", newDeviceID, &newGroup.ID, m.SortOrder); err != nil {
			return nil, fmt.Errorf("restoring device group membership for device %q: %w", newDeviceID, err)
		}
	}

	assignmentsByDevice := map[string][]int64{}
	for _, a := range bundle.DeviceTagAssignments {
		newDeviceID, ok := deviceByOldID[a.SubjectID]
		if !ok {
			continue
		}
		newTag, ok := tagByOldID[a.TagID]
		if !ok {
			continue
		}
		assignmentsByDevice[newDeviceID] = append(assignmentsByDevice[newDeviceID], newTag.ID)
	}
	for deviceID, tagIDs := range assignmentsByDevice {
		if err := repos.Tags.ReplaceForSubject(ctx, "device", deviceID, tagIDs); err != nil {
			return nil, fmt.Errorf("restoring tag assignments for device %q: %w", deviceID, err)
		}
	}

	// Alert rules last -- remap their notification_channel_ids to the newly
	// imported channels' ids (an id referencing a channel that wasn't
	// imported, e.g. skipped in "skip" mode, is dropped rather than left
	// dangling).
	for _, rule := range bundle.AlertRules {
		if mode == ModeSkip {
			exists, err := repos.Alerts.RuleExistsByName(ctx, rule.Name)
			if err != nil {
				return nil, fmt.Errorf("checking alert rule %q: %w", rule.Name, err)
			}
			if exists {
				result.AlertRulesSkipped++
				continue
			}
		}
		toCreate := rule
		toCreate.ID = 0
		remapped := make([]int64, 0, len(rule.NotificationChannelIDs))
		for _, oldID := range rule.NotificationChannelIDs {
			if newID, ok := channelByOldID[oldID]; ok {
				remapped = append(remapped, newID)
			}
		}
		toCreate.NotificationChannelIDs = remapped
		if _, err := repos.Alerts.SaveRule(ctx, toCreate); err != nil {
			return nil, fmt.Errorf("importing alert rule %q: %w", rule.Name, err)
		}
		result.AlertRulesImported++
	}

	return result, nil
}

// uniqueDeviceName appends " (import)", then " (import 2)", " (import 3)",
// ... until it finds a name not already used in the tenant -- devices has a
// UNIQUE(organization_id,name) constraint that "keep" mode's "import
// everything as new, no dedup" would otherwise violate on a name collision.
func uniqueDeviceName(ctx context.Context, repo devices.Repository, tenantID, name string) string {
	candidate := name + " (import)"
	for n := 2; ; n++ {
		exists, err := repo.ExistsByName(ctx, tenantID, candidate)
		if err != nil || !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s (import %d)", name, n)
	}
}

// importDevice creates one device from its exported Record under the given
// name, restoring every optional check (HTTP/ICMP/DNS/push/SSH/Telnet) but
// deliberately leaving SNMP disabled and provisioning-template assignment
// cleared -- see this package's doc comment for SNMP, and provisioning
// templates aren't part of the bundle's scope (a template id from the
// source tenant may not exist, or may mean something else, on the target).
// If the source device was enabled, the restored device is created enabled
// too and left running -- mirroring Kuma, which starts an imported monitor
// iff it was active in the backup.
func importDevice(ctx context.Context, repo devices.Repository, tenantID, name string, d devices.Record) (devices.Record, error) {
	created, err := repo.Create(ctx, devices.DeviceInput{
		OrganizationID: tenantID,
		Name:           name,
		Address:        d.Address,
		DeviceType:     d.DeviceType,
		Vendor:         d.Vendor,
		SerialNumber:   d.SerialNumber,
	})
	if err != nil {
		return devices.Record{}, err
	}
	if !d.Enabled {
		// Create always inserts enabled=true; explicitly pause it if the
		// source device wasn't active, mirroring Kuma's "start iff active
		// in the backup, otherwise leave paused" restore behavior.
		if err := repo.SetEnabled(ctx, created.ID, false); err != nil {
			return devices.Record{}, fmt.Errorf("restoring paused state: %w", err)
		}
	}

	if d.HTTPCheckEnabled {
		if err := repo.UpdateHTTPCheck(ctx, created.ID, devices.HTTPCheckRequest{
			Enabled: true, URL: d.HTTPURL, ExpectedStatus: d.HTTPExpectedStatus, Keyword: d.HTTPKeyword, TimeoutMS: d.HTTPTimeoutMS,
		}); err != nil {
			return devices.Record{}, fmt.Errorf("restoring HTTP check: %w", err)
		}
	}
	if d.ICMPEnabled {
		if err := repo.UpdateICMPCheck(ctx, created.ID, devices.ICMPCheckRequest{
			Enabled: true, IntervalSeconds: d.ICMPIntervalSeconds, PacketSize: d.ICMPPacketSize, Count: d.ICMPCount, Retries: d.ICMPRetries,
		}); err != nil {
			return devices.Record{}, fmt.Errorf("restoring ICMP check: %w", err)
		}
	}
	if d.DNSEnabled {
		if err := repo.UpdateDNSCheck(ctx, created.ID, devices.DNSCheckRequest{
			Enabled: true, Hostname: d.DNSHostname, RecordType: d.DNSRecordType, ResolverServer: d.DNSResolverServer, ExpectedAnswer: d.DNSExpectedAnswer, IntervalSeconds: d.DNSIntervalSeconds,
		}); err != nil {
			return devices.Record{}, fmt.Errorf("restoring DNS check: %w", err)
		}
	}
	if d.SSHEnabled {
		if err := repo.UpdateSSHCheck(ctx, created.ID, devices.SSHCheckRequest{
			Enabled: true, Port: d.SSHPort, BannerKeyword: d.SSHBannerKeyword, TimeoutMS: d.SSHTimeoutMS, IntervalSeconds: d.SSHIntervalSeconds,
		}); err != nil {
			return devices.Record{}, fmt.Errorf("restoring SSH check: %w", err)
		}
	}
	if d.TelnetEnabled {
		if err := repo.UpdateTelnetCheck(ctx, created.ID, devices.TelnetCheckRequest{
			Enabled: true, Port: d.TelnetPort, BannerKeyword: d.TelnetBannerKeyword, TimeoutMS: d.TelnetTimeoutMS, IntervalSeconds: d.TelnetIntervalSeconds,
		}); err != nil {
			return devices.Record{}, fmt.Errorf("restoring Telnet check: %w", err)
		}
	}
	if d.PushEnabled {
		if _, err := repo.UpdatePushCheck(ctx, created.ID, devices.PushCheckRequest{
			Enabled: true, IntervalSeconds: d.PushIntervalSeconds, GracePeriodSeconds: d.PushGracePeriodSeconds,
		}); err != nil {
			return devices.Record{}, fmt.Errorf("restoring push check: %w", err)
		}
	}

	final, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		return created, nil // best-effort re-read; the create itself already succeeded
	}
	return final, nil
}
