package olt

import "time"

type Status string
const (
	Online Status = "online"
	Offline Status = "offline"
	Unknown Status = "unknown"
)

type OLT struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Address string `json:"address"`
	Vendor string `json:"vendor"`
	Model string `json:"model,omitempty"`
	Serial string `json:"serial,omitempty"`
	Enabled bool `json:"enabled"`
}

type PONPort struct {
	ID string `json:"id"`
	OLTID string `json:"oltId"`
	Name string `json:"name"`
	Index int `json:"index"`
	Type string `json:"type"`
	Status Status `json:"status"`
	ONUCount int `json:"onuCount"`
}

type ONU struct {
	ID string `json:"id"`
	OLTID string `json:"oltId"`
	PONPortID string `json:"ponPortId"`
	Serial string `json:"serial"`
	Name string `json:"name,omitempty"`
	Status Status `json:"status"`
	RxPowerDBm *float64 `json:"rxPowerDbm,omitempty"`
	TxPowerDBm *float64 `json:"txPowerDbm,omitempty"`
	DistanceMeters *float64 `json:"distanceMeters,omitempty"`
	LastSeen *time.Time `json:"lastSeen,omitempty"`
	LOS bool `json:"los"`
}

type PollResult struct {
	PONs []PONPort `json:"pons"`
	ONUs []ONU `json:"onus"`
	PolledAt time.Time `json:"polledAt"`
}
