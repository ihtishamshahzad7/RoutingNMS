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
- Deployment: Docker Compose and Ubuntu 24.04 LTS native installer

## One-click Ubuntu 24.04 installation

On a fresh Ubuntu 24.04 LTS server, run only:

```bash
curl -fsSL https://raw.githubusercontent.com/ihtishamshahzad7/RoutingNMS/main/deployments/ubuntu-24.04/install.sh | sudo bash
```

The installer automatically installs the required OS packages, Go 1.24, Node.js 22, PostgreSQL, SNMP tools, Nginx and supporting utilities; creates the RoutingNMS service account/database; downloads the application; builds the Go API and Next.js frontend; installs systemd services; configures Nginx; and runs health/readiness checks.

To supply a fixed database password instead of generating one:

```bash
curl -fsSL https://raw.githubusercontent.com/ihtishamshahzad7/RoutingNMS/main/deployments/ubuntu-24.04/install.sh | sudo ROUTINGNMS_DB_PASSWORD='CHANGE_THIS_PASSWORD' bash
```

### Accessing RoutingNMS

Open `http://SERVER-IP/` (port 80, no port suffix) in your browser. Nginx is
the only intended entry point: it proxies `/api/v1/...` to the Go backend
on `127.0.0.1:8080` and everything else to the Next.js frontend on
`127.0.0.1:3000`. Both of those ports are bound to loopback only and are
not reachable directly from another machine — going straight to
`http://SERVER-IP:3000/` will load the page shell but every API call
(including login) will fail, since Next.js itself has no route for
`/api/v1/...`.

The installer is intentionally restricted to Ubuntu 24.04 LTS so unsupported OS/package combinations fail early instead of producing a partially working deployment.

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
├── deployments/   # Docker/production deployment and Ubuntu installer
├── docs/          # Architecture and operational documentation
└── .env.example
```

This repository is being built incrementally. Each phase must preserve production-quality security, observability and backward compatibility.
