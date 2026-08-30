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
[[ -r /etc/os-release ]] || fail "Cannot detect Ubuntu release: /etc/os-release is missing."
. /etc/os-release

if [[ "${ID:-}" != "ubuntu" ]]; then
  fail "RoutingNMS requires Ubuntu; detected ${PRETTY_NAME:-unknown}."
fi
case "${VERSION_ID:-}" in
  22.04) UBUNTU_CODENAME="jammy"; UBUNTU_MAJOR="22.04" ;;
  24.04) UBUNTU_CODENAME="noble"; UBUNTU_MAJOR="24.04" ;;
  26.04) UBUNTU_CODENAME="resolute"; UBUNTU_MAJOR="26.04" ;;
  *) fail "Unsupported Ubuntu release ${PRETTY_NAME:-unknown} (VERSION_ID=${VERSION_ID:-unknown}). Supported releases: Ubuntu 22.04.x, 24.04.x and 26.04.x LTS." ;;
esac
export DEBIAN_FRONTEND=noninteractive

log "Detected supported Ubuntu ${UBUNTU_MAJOR}.x (${UBUNTU_CODENAME})"
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
if ! sudo -u postgres -H sh -c 'cd / && psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='"'"'routingnms'"'"'"' | grep -q 1; then
  sudo -u postgres -H sh -c "cd / && psql -v ON_ERROR_STOP=1 -c \"CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASSWORD}'\""
else
  sudo -u postgres -H sh -c "cd / && psql -v ON_ERROR_STOP=1 -c \"ALTER ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASSWORD}'\""
fi
if ! sudo -u postgres -H sh -c 'cd / && psql -tAc "SELECT 1 FROM pg_database WHERE datname='"'"'routingnms'"'"'"' | grep -q 1; then
  sudo -u postgres -H sh -c "cd / && createdb -O '${DB_USER}' '${DB_NAME}'"
else
  sudo -u postgres -H sh -c "cd / && psql -v ON_ERROR_STOP=1 -c \"ALTER DATABASE ${DB_NAME} OWNER TO ${DB_USER}\""
fi

log "Downloading RoutingNMS"
if [[ -d "$APP_DIR/.git" ]]; then
  git -c safe.directory="$APP_DIR" -C "$APP_DIR" fetch --all --prune
  git -c safe.directory="$APP_DIR" -C "$APP_DIR" reset --hard origin/main
else
  rm -rf "$APP_DIR"
  git clone --depth 1 "$REPO_URL" "$APP_DIR"
fi
chown -R "$APP_USER:$APP_USER" "$APP_DIR"

log "Synchronizing Go dependencies"
sudo -u "$APP_USER" env HOME="$APP_DIR" PATH="/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin" bash -c 'cd /opt/routingnms/backend && go mod tidy && go mod download && go mod verify'

log "Building backend"
sudo -u "$APP_USER" env HOME="$APP_DIR" PATH="/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin" bash -c 'cd /opt/routingnms/backend && CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /opt/routingnms/routingnms-api ./cmd/api'

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
printf '\nDetected Ubuntu: %s.x (%s)\nOpen: http://SERVER-IP/\nInstall: %s\n\n' "$UBUNTU_MAJOR" "$UBUNTU_CODENAME" "$APP_DIR"
