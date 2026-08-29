package olt

import "testing"

func TestONUIndexSpecExtract(t *testing.T) {
	s := ONUIndexSpec{PONPositions: []int{0}, ONUPositions: []int{0, 1}, Separator: "."}
	pon, onu, err := s.Extract("3.17")
	if err != nil { t.Fatalf("Extract returned error: %v", err) }
	if pon != "3" { t.Fatalf("PON index = %q, want %q", pon, "3") }
	if onu != "3.17" { t.Fatalf("ONU index = %q, want %q", onu, "3.17") }
}

func TestONUIndexSpecRejectsInvalidPosition(t *testing.T) {
	s := ONUIndexSpec{PONPositions: []int{2}, ONUPositions: []int{0}, Separator: "."}
	if _, _, err := s.Extract("3.17"); err == nil { t.Fatal("expected invalid position error") }
}

func TestOIDTemplateIndexed(t *testing.T) {
	tpl := OIDTemplate{Base: "1.3.6.1.4.1.999", IndexOrder: []string{"pon", "onu"}}
	got, err := tpl.Indexed("3", "17")
	if err != nil { t.Fatalf("Indexed returned error: %v", err) }
	want := "1.3.6.1.4.1.999.3.17"
	if got != want { t.Fatalf("OID = %q, want %q", got, want) }
}
