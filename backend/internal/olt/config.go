package olt

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Config contains environment-driven OLT polling defaults. Device inventory
// can later come from the database/API without changing the poller contract.
type Config struct {
	Enabled bool
	Interval time.Duration
	Target snmp.Target
}

func LoadConfig() (Config, error) {
	interval := 60 * time.Second
	if v := strings.TrimSpace(os.Getenv("OLT_POLL_INTERVAL")); v != "" {
		d, err := time.ParseDuration(v); if err != nil || d < 30*time.Second { return Config{}, fmt.Errorf("invalid OLT_POLL_INTERVAL: %q", v) }; interval = d
	}
	port := uint16(161)
	if v := strings.TrimSpace(os.Getenv("OLT_SNMP_PORT")); v != "" { n,err:=strconv.Atoi(v); if err!=nil || n<1 || n>65535{return Config{},fmt.Errorf("invalid OLT_SNMP_PORT")}; port=uint16(n) }
	timeout := 5*time.Second
	if v:=strings.TrimSpace(os.Getenv("OLT_SNMP_TIMEOUT"));v!="" { d,err:=time.ParseDuration(v);if err!=nil||d<=0{return Config{},fmt.Errorf("invalid OLT_SNMP_TIMEOUT")};timeout=d }
	community:=os.Getenv("OLT_SNMP_COMMUNITY")
	return Config{Enabled: strings.EqualFold(os.Getenv("OLT_POLL_ENABLED"),"true"), Interval:interval, Target:snmp.Target{Address:os.Getenv("OLT_SNMP_ADDRESS"),Port:port,Timeout:timeout,Retries:2,Credentials:snmp.Credentials{Version:snmp.V2c,Community:community}}},nil
}
