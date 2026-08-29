package olt

import "time"

// Vendor-neutral OLT inventory. Vendor-specific adapters map SNMP/API/CLI
// responses into these models.
type OLT struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Vendor      string `json:"vendor"`
	Model       string `json:"model,omitempty"`
	Serial      string `json:"serial,omitempty"`
	Enabled     bool   `json:"enabled"`
}

type PONPort struct {
	ID       string `json:"id"`
	OLTID    string `json:"oltId"`
	Name     string `json:"name"`
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

type ONU struct {
	ID          string     `json:"id"`
	OLTID       string     `json:"oltId"`
	PONPortID   string     `json:"ponPortId"`
	Serial      string     `json:"serial"`
	Name        string     `json:"name,omitempty"`
	Status      string     `json:"status"`
	RxPowerDBm  *float64   `json:"rxPowerDbm,omitempty"`
	TxPowerDBm  *float64   `json:"txPowerDbm,omitempty"`
	LastSeen    *time.Time `json:"lastSeen,omitempty"`
	LOS         bool       `json:"los"`
}
