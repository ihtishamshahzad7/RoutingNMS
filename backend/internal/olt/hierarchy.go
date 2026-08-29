package olt

import "time"

// PON is the API-facing hierarchy node. ONU is the canonical domain model
// defined in model.go, avoiding duplicate ONU definitions with drifting fields.
type PON struct {
	ID string `json:"id"`
	Port int `json:"port"`
	Status Status `json:"status"`
	ONUs []ONU `json:"onus"`
}

type Hierarchy struct {
	OLTID string `json:"oltId"`
	Name string `json:"name"`
	Model string `json:"model,omitempty"`
	PONs []PON `json:"pons"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewHierarchy(id, name, model string, pons []PON) Hierarchy {
	return Hierarchy{OLTID:id, Name:name, Model:model, PONs:pons, UpdatedAt:time.Now().UTC()}
}
