# RoutingNMS

AI-powered Network Operations Management System for ISPs, MSPs and enterprises.

## Architecture

- Frontend: Next.js + TypeScript + Tailwind CSS
- Backend: Go
- Core database: PostgreSQL
- Time-series metrics: VictoriaMetrics
- Cache/state: Redis
- Event/worker bus: NATS JetStream
- Monitoring: ICMP + SNMP v2c/v3
- Syslog: planned Go collector
- Topology: LLDP/CDP + SNMP
- Deployment: Docker Compose

## Design goals

- ISP/OLT focused
- Works over slow and unreliable links
- Multi-tenant and RBAC-ready
- Horizontally scalable polling workers
- Secure secret handling
- Real-time events through WebSockets
- AI-assisted incident analysis and RCA

## Repository layout

```text
RoutingNMS/
├── backend/       # Go API and NMS services
├── frontend/      # Next.js web application
├── deployments/   # Docker/production deployment
├── docs/          # Architecture and operational documentation
└── .env.example
```

This repository is being built incrementally. Each phase must preserve production-quality security, observability and backward compatibility.