package olt

import "time"

type ONU struct { ID string `json:"id"`; Name string `json:"name"`; Serial string `json:"serial"`; Status string `json:"status"`; RxPowerDbm *float64 `json:"rxPowerDbm,omitempty"`; TxPowerDbm *float64 `json:"txPowerDbm,omitempty"` }
type PON struct { ID string `json:"id"`; Port int `json:"port"`; Status string `json:"status"`; ONUs []ONU `json:"onus"` }
type Hierarchy struct { OLTID string `json:"oltId"`; Name string `json:"name"`; Model string `json:"model,omitempty"`; PONs []PON `json:"pons"`; UpdatedAt time.Time `json:"updatedAt"` }

func NewHierarchy(id,name,model string, pons []PON) Hierarchy { return Hierarchy{OLTID:id,Name:name,Model:model,PONs:pons,UpdatedAt:time.Now().UTC()} }
