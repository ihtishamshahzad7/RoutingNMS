# Monitoring engine

The monitoring domain is designed around pluggable probes. The first probe is an unprivileged TCP reachability check. Future workers will add ICMP, SNMP v2c/v3, HTTP, DNS, SSH and vendor-specific OLT adapters.

## ISP/OLT design goals

- Per-device polling intervals and timeouts.
- Bounded concurrency so a slow OLT never blocks the entire poller.
- Retries with backoff for unstable links.
- Vendor-neutral device/PON/ONU models.
- Metric batching into the time-series backend.
- Event correlation before alert creation.
- Credentials remain server-side and are never returned to the UI or AI layer.
