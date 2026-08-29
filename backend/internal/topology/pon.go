package topology

// PONNode represents an ISP fiber access hierarchy below an OLT. Keeping this
// separate from generic Node lets future OLT adapters attach vendor-specific
// optical data without polluting the core topology graph.
type PONNode struct {
	ID string `json:"id"`
	OLTID string `json:"oltId"`
	Port string `json:"port"`
	Name string `json:"name"`
	Status LinkStatus `json:"status"`
	ONUCount int `json:"onuCount"`
}

type ONUNode struct {
	ID string `json:"id"`
	PONID string `json:"ponId"`
	Serial string `json:"serial"`
	Name string `json:"name,omitempty"`
	Status LinkStatus `json:"status"`
	RxPowerDbm float64 `json:"rxPowerDbm,omitempty"`
	TxPowerDbm float64 `json:"txPowerDbm,omitempty"`
}

type PONHierarchy struct {
	OLT Node `json:"olt"`
	PONs []PONNode `json:"pons"`
	ONUs []ONUNode `json:"onus"`
}

func BuildPONHierarchy(olt Node, pons []PONNode, onus []ONUNode) PONHierarchy {
	return PONHierarchy{OLT: olt, PONs: pons, ONUs: onus}
}
