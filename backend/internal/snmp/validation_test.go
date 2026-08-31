package snmp

import (
	"testing"
	"time"
)

func TestTargetNormalizeAndValidate(t *testing.T) {
	target := Target{Address: " 192.0.2.10 ", Credentials: Credentials{Version: "v2c", Community: "public"}}
	normalized := target.Normalize()
	if normalized.Address != "192.0.2.10" {
		t.Fatalf("Address = %q, want trimmed address", normalized.Address)
	}
	if normalized.Port != DefaultPort {
		t.Fatalf("Port = %d, want %d", normalized.Port, DefaultPort)
	}
	if normalized.Timeout != DefaultTimeout {
		t.Fatalf("Timeout = %s, want %s", normalized.Timeout, DefaultTimeout)
	}
	if err := normalized.Validate(); err != nil {
		t.Fatalf("valid v2c target rejected: %v", err)
	}
}

func TestTargetValidateRejectsMissingCredentials(t *testing.T) {
	cases := []Target{
		{Address: "192.0.2.10", Credentials: Credentials{Version: V2c}},
		{Address: "192.0.2.10", Credentials: Credentials{Version: V3}},
	}
	for _, target := range cases {
		if err := target.Validate(); err == nil {
			t.Fatal("target without required credentials was accepted")
		}
	}
}

func TestTargetValidateRejectsInvalidTimeout(t *testing.T) {
	target := Target{
		Address:  "192.0.2.10",
		Timeout:  -time.Second,
		Retries:  1,
		Credentials: Credentials{Version: V2c, Community: "public"},
	}
	if err := target.Validate(); err == nil {
		t.Fatal("negative timeout was accepted")
	}
}
