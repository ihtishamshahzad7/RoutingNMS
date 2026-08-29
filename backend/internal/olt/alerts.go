package olt

import (
	"fmt"
	"math"
)

type OpticalAlert struct {
	Code string `json:"code"`
	Severity string `json:"severity"`
	ONU string `json:"onu"`
	Message string `json:"message"`
	Value *float64 `json:"value,omitempty"`
}

type OpticalPolicy struct {
	MinRXDBm float64
	MaxRXDBm float64
	MaxTXDBm float64
}

// EvaluateONU applies conservative optical thresholds. Thresholds are
// configurable because optics and OLT vendor specifications differ.
func EvaluateONU(onu ONU, policy OpticalPolicy) []OpticalAlert {
	alerts := []OpticalAlert{}
	if onu.LOS { alerts = append(alerts, OpticalAlert{Code:"onu_los",Severity:"critical",ONU:onu.ID,Message:"ONU reports loss of signal"}) }
	if onu.Status == Offline { alerts = append(alerts, OpticalAlert{Code:"onu_offline",Severity:"warning",ONU:onu.ID,Message:"ONU is offline"}) }
	if onu.RXPowerDBm != nil {
		v := *onu.RXPowerDBm
		if v < policy.MinRXDBm { alerts = append(alerts, OpticalAlert{Code:"low_rx_power",Severity:"critical",ONU:onu.ID,Message:fmt.Sprintf("ONU RX power is below threshold (%.2f dBm)",v),Value:&v}) }
		if v > policy.MaxRXDBm { alerts = append(alerts, OpticalAlert{Code:"high_rx_power",Severity:"warning",ONU:onu.ID,Message:fmt.Sprintf("ONU RX power is above threshold (%.2f dBm)",v),Value:&v}) }
	}
	if onu.TXPowerDBm != nil && *onu.TXPowerDBm > policy.MaxTXDBm { v:=*onu.TXPowerDBm; alerts=append(alerts,OpticalAlert{Code:"high_tx_power",Severity:"warning",ONU:onu.ID,Message:"ONU TX power is above threshold",Value:&v}) }
	return alerts
}

// MassOutage detects a PON-wide event so NOC operators get one root-cause
// alert instead of hundreds of individual ONU notifications.
func MassOutage(pon PON, onus []ONU, minimumCount int, offlineRatio float64) bool {
	if len(onus) < minimumCount || offlineRatio <= 0 || offlineRatio > 1 { return false }
	offline := 0
	for _, o := range onus { if o.Status == Offline || o.LOS { offline++ } }
	return math.Abs(float64(offline)/float64(len(onus))) >= offlineRatio
}
