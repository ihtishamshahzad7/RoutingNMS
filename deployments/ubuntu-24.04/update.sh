#!/usr/bin/env bash
set -Eeuo pipefail

APP_DIR="/opt/routingnms"
APP_USER="routingnms"
API_PORT="8080"
WEB_PORT="3000"
REPO_URL="${ROUTINGNMS_REPO_URL:-https://github.com/ihtishamshahzad7/RoutingNMS.git}"

log(){ printf '\n[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
fail(){ echo "ERROR: $*" >&2; exit 1; }
[[ "$(id -u)" -eq 0 ]] || fail "Run as root (sudo -i)."
[[ -d "$APP_DIR/.git" ]] || fail "$APP_DIR is not a Git checkout. Run install.sh first."

log "Pulling latest RoutingNMS code"
git -c safe.directory="$APP_DIR" -C "$APP_DIR" remote set-url origin "$REPO_URL"
git -c safe.directory="$APP_DIR" -C "$APP_DIR" fetch origin
GIT_OLD="$(git -c safe.directory="$APP_DIR" -C "$APP_DIR" rev-parse HEAD)"
git -c safe.directory="$APP_DIR" -C "$APP_DIR" reset --hard origin/main
GIT_NEW="$(git -c safe.directory="$APP_DIR" -C "$APP_DIR" rev-parse HEAD)"
printf 'Updated %s -> %s\n' "${GIT_OLD:0:12}" "${GIT_NEW:0:12}"

# The repository is deliberately owned by root. Do dependency resolution/builds as root
# so Go/npm can update module/package metadata and write build caches without permission errors.
log "Synchronizing Go dependencies and building backend"
cd "$APP_DIR/backend"
go mod tidy
go mod download
go mod verify
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" \
  -o "$APP_DIR/routingnms-api" ./cmd/api

if [[ -f "$APP_DIR/frontend/package.json" ]]; then
  log "Installing frontend dependencies and building Next.js"
  cd "$APP_DIR/frontend"
  npm install --no-audit --no-fund
  npm run build
fi

log "Applying PostgreSQL migrations"
if [[ -d "$APP_DIR/backend/migrations" ]]; then
  shopt -s nullglob
  migrations=("$APP_DIR/backend/migrations"/*.sql)
  for migration in "${migrations[@]}"; do
    log "Applying $(basename "$migration")"
    sudo -u postgres psql -d routingnms -v ON_ERROR_STOP=1 -f "$migration"
  done
  sudo -u postgres psql -d routingnms -v ON_ERROR_STOP=1 -c "GRANT USAGE ON SCHEMA public TO ${APP_USER}; GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${APP_USER}; GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ${APP_USER}; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${APP_USER}; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO ${APP_USER};"
fi

log "Installing current systemd and Nginx configuration"
install -m 0644 "$APP_DIR/deployments/ubuntu-24.04/routingnms-api.service" /etc/systemd/system/routingnms-api.service
install -m 0644 "$APP_DIR/deployments/ubuntu-24.04/routingnms-web.service" /etc/systemd/system/routingnms-web.service
install -m 0644 "$APP_DIR/deployments/ubuntu-24.04/nginx.conf" /etc/nginx/sites-available/routingnms
ln -sfn /etc/nginx/sites-available/routingnms /etc/nginx/sites-enabled/routingnms
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl daemon-reload
systemctl enable --now routingnms-api routingnms-web nginx
systemctl restart routingnms-api routingnms-web
systemctl reload nginx

log "Checking API and frontend"
curl -fsS "http://127.0.0.1:${API_PORT}/api/v1/health" | jq .
curl -fsS "http://127.0.0.1:${API_PORT}/api/v1/ready" | jq .
curl -fsSI "http://127.0.0.1:${WEB_PORT}/" | head -n 12

log "Checking Next.js static assets through Nginx"
ASSET_PATH="$(curl -fsS http://127.0.0.1:${WEB_PORT}/ | grep -oE '/_next/static/[^\" ]+\.(css|js)' | head -n 1 || true)"
[[ -n "$ASSET_PATH" ]] || fail "Could not find a CSS/JS asset in the Next.js HTML."
curl -fsSI "http://127.0.0.1${ASSET_PATH}" | head -n 12

SERVER_IP="$(hostname -I | awk '{print $1}')"
[[ -n "$SERVER_IP" ]] || SERVER_IP="SERVER-IP"

log "RoutingNMS update completed successfully"
printf '\nVersion commit: %s\nLogin: http://%s/\nDashboard: http://%s/dashboard\nDefault username: admin\nDefault password: admin123\n\n' "$GIT_NEW" "$SERVER_IP" "$SERVER_IP" 
