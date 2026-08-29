package olt

import "fmt"

type OpticalAlert struct {
	Code string `json:"code"`
	Severity string `json:"severity"`
	ONU string `json:"onu"`
	Message string `json:"message"`
	Value *float64 `json:"value,omitempty"`
}

type OpticalPolicy struct {
	MinRXDBm float64 `json:"minRxDbm"`
	MaxRXDBm float64 `json:"maxRxDbm"`
	MaxTXDBm float64 `json:"maxTxDbm"`
}

func DefaultOpticalPolicy() OpticalPolicy { return OpticalPolicy{MinRXDBm:-28,MaxRXDBm:-8,MaxTXDBm:5} }

// EvaluateONU evaluates link state and optical thresholds. Thresholds are
// supplied by the caller because optics and vendor specifications differ.
func EvaluateONU(onu ONU, policy OpticalPolicy) []OpticalAlert {
	alerts:=[]OpticalAlert{}
	if onu.LOS { alerts=append(alerts,OpticalAlert{Code:"onu_los",Severity:"critical",ONU:onu.ID,Message:"ONU reports loss of signal"}) }
	if onu.Status==Offline { alerts=append(alerts,OpticalAlert{Code:"onu_offline",Severity:"warning",ONU:onu.ID,Message:"ONU is offline"}) }
	if onu.RxPowerDBm!=nil {
		v:=*onu.RxPowerDBm
		if v<policy.MinRXDBm { alerts=append(alerts,OpticalAlert{Code:"low_rx_power",Severity:"critical",ONU:onu.ID,Message:fmt.Sprintf("ONU RX power is below threshold (%.2f dBm)",v),Value:&v}) }
		if v>policy.MaxRXDBm { alerts=append(alerts,OpticalAlert{Code:"high_rx_power",Severity:"warning",ONU:onu.ID,Message:fmt.Sprintf("ONU RX power is above threshold (%.2f dBm)",v),Value:&v}) }
	}
	if onu.TxPowerDBm!=nil && *onu.TxPowerDBm>policy.MaxTXDBm { v:=*onu.TxPowerDBm; alerts=append(alerts,OpticalAlert{Code:"high_tx_power",Severity:"warning",ONU:onu.ID,Message:fmt.Sprintf("ONU TX power is above threshold (%.2f dBm)",v),Value:&v}) }
	return alerts
}

// MassOutage detects a PON-wide outage from its actual PONPort identity.
func MassOutage(pon PONPort, onus []ONU, minimumCount int, offlineRatio float64) bool {
	if len(onus)<minimumCount || offlineRatio<=0 || offlineRatio>1 { return false }
	offline:=0
	for _,o:=range onus { if o.Status==Offline || o.LOS { offline++ } }
	return float64(offline)/float64(len(onus))>=offlineRatio
}
