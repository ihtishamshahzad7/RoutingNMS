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
  # IMPORTANT: this is the historical root cause of "Next.js builds fine but
  # the site renders as unstyled raw HTML". Tailwind (@tailwindcss/postcss,
  # tailwindcss) lives in devDependencies. If NODE_ENV=production is set
  # anywhere in this shell's environment (a shell profile, a CI default, an
  # inherited systemd Environment=), a plain `npm install`/`npm ci` silently
  # skips devDependencies. The Next.js build still succeeds — it just emits
  # a near-empty stylesheet because the Tailwind PostCSS plugin was never
  # installed. Explicitly forcing --include=dev makes the frontend build
  # correct regardless of what NODE_ENV happens to be set to.
  if [[ -f package-lock.json ]]; then
    npm ci --include=dev --no-audit --no-fund
  else
    npm install --include=dev --no-audit --no-fund
  fi
  npm run build

  log "Verifying the Next.js production build actually contains Tailwind output"
  CSS_FILE="$(find .next/static/css -maxdepth 1 -name '*.css' -print -quit 2>/dev/null || true)"
  [[ -n "$CSS_FILE" ]] || fail "Next.js build produced no CSS file under .next/static/css/. Frontend build is broken."
  CSS_BYTES="$(wc -c < "$CSS_FILE")"
  if [[ "$CSS_BYTES" -lt 1000 ]]; then
    fail "Generated CSS ($CSS_FILE) is only ${CSS_BYTES} bytes. Tailwind utilities were not compiled — check that @tailwindcss/postcss installed (devDependencies must not be skipped)."
  fi
  log "Generated CSS looks correct (${CSS_BYTES} bytes): $CSS_FILE"
fi

log "Applying PostgreSQL migrations"
if [[ -d "$APP_DIR/backend/migrations" ]]; then
  shopt -s nullglob
  # Sorted so numeric prefixes apply in the intended order (0001, 0003,
  # 0004, 0005, ...) regardless of how many digits each file happens to use.
  migrations=($(printf '%s\n' "$APP_DIR/backend/migrations"/*.sql | sort -V))
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

log "Checking Next.js static CSS and JavaScript through Nginx"
HTML="$(curl -fsS http://127.0.0.1:${WEB_PORT}/)"
CSS_PATH="$(printf '%s' "$HTML" | grep -oE '/_next/static/[^\" ]+\.css' | head -n 1 || true)"
JS_PATH="$(printf '%s' "$HTML" | grep -oE '/_next/static/[^\" ]+\.js' | head -n 1 || true)"
[[ -n "$CSS_PATH" ]] || fail "Could not find a CSS asset in the Next.js HTML."
[[ -n "$JS_PATH" ]] || fail "Could not find a JavaScript asset in the Next.js HTML."

CSS_HEADERS="$(curl -fsSI "http://127.0.0.1${CSS_PATH}")" || fail "Next.js CSS asset is not reachable through Nginx: ${CSS_PATH}"
JS_HEADERS="$(curl -fsSI "http://127.0.0.1${JS_PATH}")" || fail "Next.js JavaScript asset is not reachable through Nginx: ${JS_PATH}"

printf '%s\n' "$CSS_HEADERS" | head -n 12
printf '%s\n' "$JS_HEADERS" | head -n 12

printf '%s\n' "$CSS_HEADERS" | grep -qiE '200 OK' || fail "CSS asset did not return HTTP 200: ${CSS_PATH}"
printf '%s\n' "$JS_HEADERS" | grep -qiE '200 OK' || fail "JavaScript asset did not return HTTP 200: ${JS_PATH}"
printf '%s\n' "$CSS_HEADERS" | grep -qiE 'text/css' || fail "CSS asset has an unexpected Content-Type: ${CSS_PATH}"

# A Tailwind-enabled production build should contain substantially more than the
# tiny base stylesheet. This catches deployments where utility classes are present
# in JSX but Tailwind/PostCSS is missing from the build pipeline.
CSS_LENGTH="$(printf '%s\n' "$CSS_HEADERS" | awk -F': ' 'tolower($1)=="content-length" {print $2}' | tr -d '\r' | tail -n 1)"
if [[ -n "$CSS_LENGTH" && "$CSS_LENGTH" -lt 1000 ]]; then
  fail "Production CSS is only ${CSS_LENGTH} bytes. Tailwind utilities were probably not compiled."
fi

log "Checking authentication endpoint"
curl -fsS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${API_PORT}/api/v1/auth/me" | grep -qE '^(200|401)$' \
  || fail "auth/me did not respond as expected (expected 200 or 401)."

log "Verifying systemd services are active"
for svc in routingnms-api routingnms-web nginx; do
  systemctl is-active --quiet "$svc" || fail "Service $svc is not active."
done

SERVER_IP="$(hostname -I | awk '{print $1}')"
[[ -n "$SERVER_IP" ]] || SERVER_IP="SERVER-IP"

printf '\n==================================================\n'
printf 'RoutingNMS UPDATE COMPLETE\n'
printf '==================================================\n'
printf 'Version:\n%s\n\n' "$GIT_NEW"
printf 'Server:\n%s\n\n' "$SERVER_IP"
printf 'Login:\nhttp://%s/\n\n' "$SERVER_IP"
printf 'Dashboard:\nhttp://%s/dashboard\n\n' "$SERVER_IP"
printf 'API:\nhttp://%s:%s\n\n' "$SERVER_IP" "$API_PORT"
printf 'Default username:\nadmin\n\n'
printf 'Default password:\nadmin123\n(change this after first login)\n\n'
printf 'Services:\nroutingnms-api: OK\nroutingnms-web: OK\nnginx: OK\n\n'
printf 'PostgreSQL: OK\nFrontend CSS: OK\nFrontend JS: OK\n'
printf '==================================================\n\n'
