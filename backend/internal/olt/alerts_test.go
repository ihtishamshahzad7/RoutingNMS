package olt

import "testing"

func TestEvaluateONUDetectsLOS(t *testing.T) {
	o := ONU{ID: "onu-1", LOS: true}
	a := EvaluateONU(o, DefaultOpticalPolicy())
	if len(a) != 1 || a[0].Code != "onu_los" || a[0].Severity != "critical" { t.Fatalf("unexpected alerts: %#v", a) }
}

func TestEvaluateONUDetectsLowRX(t *testing.T) {
	v := -30.0
	o := ONU{ID: "onu-1", RxPowerDBm: &v}
	a := EvaluateONU(o, DefaultOpticalPolicy())
	found := false
	for _, x := range a { if x.Code == "low_rx_power" { found = true } }
	if !found { t.Fatalf("expected low_rx_power alert: %#v", a) }
}

func TestMassOutage(t *testing.T) {
	onus := []ONU{{ID:"1",Status:Offline},{ID:"2",Status:Offline},{ID:"3",Status:Online},{ID:"4",Status:Offline}}
	if !MassOutage(PONPort{ID:"pon-1"}, onus, 4, 0.75) { t.Fatal("expected mass outage") }
}
