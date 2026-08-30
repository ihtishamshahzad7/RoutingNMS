# SNMP package

The package supports SNMP v2c and v3 targets through the collector. Target configuration is normalized and validated before a connection is opened.

## Verification

Run:

```bash
go test ./internal/snmp
```

The package tests cover target defaults/validation and the value conversion used by discovery.
