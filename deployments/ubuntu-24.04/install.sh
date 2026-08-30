#!/usr/bin/env bash
set -Eeuo pipefail

APP_DIR="/opt/routingnms"
APP_USER="routingnms"
APP_PORT="8080"
GO_VERSION="1.24.6"
NODE_MAJOR="22"
DB_NAME="routingnms"
DB_USER="routingnms"
DB_PASSWORD="${ROUTINGNMS_DB_PASSWORD:-}"
REPO_URL="${ROUTINGNMS_REPO_URL:-https://github.com/ihtishamshahzad7/RoutingNMS.git}"

log(){ printf '\n[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
fail(){ echo "ERROR: $*" >&2; exit 1; }
[[ "$(id -u)" -eq 0 ]] || fail "Run as root (sudo -i)."
. /etc/os-release
[[ "${ID:-}" == "ubuntu" && "${VERSION_ID:-}" == "24.04" ]] || fail "This installer requires Ubuntu 24.04 LTS; detected ${PRETTY_NAME:-unknown}."
export DEBIAN_FRONTEND=noninteractive

log "Installing all Ubuntu dependencies"
apt-get update
apt-get install -y ca-certificates curl git gnupg lsb-release build-essential nginx postgresql postgresql-contrib snmp snmp-mibs-downloader openssl jq rsync unzip tar sudo

log "Installing Go ${GO_VERSION}"
ARCH="$(dpkg --print-architecture)"
case "$ARCH" in amd64) GO_ARCH=amd64;; arm64) GO_ARCH=arm64;; *) fail "Unsupported architecture: $ARCH";; esac
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
ln -sfn /usr/local/go/bin/go /usr/local/bin/go
rm -f /tmp/go.tgz

log "Installing Node.js ${NODE_MAJOR}.x"
mkdir -p /etc/apt/keyrings
curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor --yes -o /etc/apt/keyrings/nodesource.gpg
echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_${NODE_MAJOR}.x nodistro main" > /etc/apt/sources.list.d/nodesource.list
apt-get update
apt-get install -y nodejs

log "Creating application account"
if ! id "$APP_USER" >/dev/null 2>&1; then useradd --system --home "$APP_DIR" --create-home --shell /usr/sbin/nologin "$APP_USER"; fi
mkdir -p "$APP_DIR"

log "Configuring PostgreSQL"
systemctl enable --now postgresql
DB_PASSWORD="${DB_PASSWORD:-$(openssl rand -hex 24)}"
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1; then
  sudo -u postgres psql -v ON_ERROR_STOP=1 -c "CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASSWORD}'"
else
  sudo -u postgres psql -v ON_ERROR_STOP=1 -c "ALTER ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASSWORD}'"
fi
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
  sudo -u postgres createdb -O "$DB_USER" "$DB_NAME"
else
  sudo -u postgres psql -v ON_ERROR_STOP=1 -c "ALTER DATABASE ${DB_NAME} OWNER TO ${DB_USER}"
fi

log "Downloading RoutingNMS"
if [[ -d "$APP_DIR/.git" ]]; then
  git -C "$APP_DIR" fetch --all --prune
  git -C "$APP_DIR" reset --hard origin/main
else
  rm -rf "$APP_DIR"
  git clone --depth 1 "$REPO_URL" "$APP_DIR"
fi
chown -R "$APP_USER:$APP_USER" "$APP_DIR"

log "Building backend"
sudo -u "$APP_USER" env HOME="$APP_DIR" PATH="/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin" bash -c 'cd /opt/routingnms/backend && go mod download && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /opt/routingnms/routingnms-api ./cmd/api'

log "Building frontend"
if [[ -f "$APP_DIR/frontend/package.json" ]]; then
  sudo -u "$APP_USER" env HOME="$APP_DIR" PATH="/usr/local/bin:/usr/bin:/bin" bash -c 'cd /opt/routingnms/frontend && npm install --no-audit --no-fund && npm run build'
fi

log "Creating production environment"
cat > "$APP_DIR/.env" <<EOF
APP_ENV=production
APP_PORT=${APP_PORT}
DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:5432/${DB_NAME}?sslmode=disable
POSTGRES_HOST=127.0.0.1
POSTGRES_PORT=5432
POSTGRES_DB=${DB_NAME}
POSTGRES_USER=${DB_USER}
POSTGRES_PASSWORD=${DB_PASSWORD}
EOF
chmod 600 "$APP_DIR/.env"
chown "$APP_USER:$APP_USER" "$APP_DIR/.env"

log "Installing services and Nginx"
install -m 0644 "$APP_DIR/deployments/ubuntu-24.04/routingnms-api.service" /etc/systemd/system/routingnms-api.service
install -m 0644 "$APP_DIR/deployments/ubuntu-24.04/routingnms-web.service" /etc/systemd/system/routingnms-web.service
install -m 0644 "$APP_DIR/deployments/ubuntu-24.04/nginx.conf" /etc/nginx/sites-available/routingnms
ln -sfn /etc/nginx/sites-available/routingnms /etc/nginx/sites-enabled/routingnms
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl daemon-reload
systemctl enable --now routingnms-api routingnms-web nginx

log "Running health checks"
sleep 2
curl -fsS "http://127.0.0.1:${APP_PORT}/api/v1/health" | jq .
curl -fsS "http://127.0.0.1:${APP_PORT}/api/v1/ready" | jq .

log "RoutingNMS is installed"
printf '\nOpen: http://SERVER-IP/\nInstall: %s\n\n' "$APP_DIR"
