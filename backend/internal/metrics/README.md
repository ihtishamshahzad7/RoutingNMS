# Metrics pipeline

RoutingNMS stores inventory and configuration in PostgreSQL while high-volume time-series samples are written to VictoriaMetrics through its Prometheus-compatible ingestion endpoint.

Keep labels bounded and stable. Never use subscriber names, free-form log messages, credentials, or other unbounded values as metric labels.

The writer accepts batches so polling workers can buffer samples and reduce network overhead, which is important for slow WAN links and large ISP deployments.
