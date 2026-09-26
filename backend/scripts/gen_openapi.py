#!/usr/bin/env python3
"""
Phase 0.4 (RoutingNMS build blueprint): API-First Discipline.

Generates backend/internal/apidocs/openapi.json from the actual route
registrations in backend/cmd/api/main.go, rather than hand-authoring an
OpenAPI spec from scratch or by memory.

Scope decision, stated plainly: with 155 unique registered routes, hand-
writing full request/response JSON Schemas for every one in this pass would
either take an enormous amount of time or (more likely) drift from the real
handlers within a few features and become actively misleading -- worse than
no spec. This generator instead produces an accurate *skeleton*: every real
method+path this server actually serves, grouped into tags by their first
path segment, each marked as requiring the existing session-cookie/API-key
auth unless it's one of the two known-public routes (health/ready). Filling
in real request/response schemas per-handler is explicit follow-up 0.4b,
and can be done incrementally per-feature from here on without redoing this
scaffold -- e.g. by adding a `requestBody`/`responses` entry by hand as each
endpoint is touched, or extending this script to also inspect the Go struct
a handler decodes/encodes.

Run this after adding/removing/renaming a route in main.go:
    python3 backend/scripts/gen_openapi.py

It is intentionally a plain script with no dependencies beyond the stdlib,
since this sandbox has no proxy.golang.org access to add a Go-side codegen
dependency, and the same is true of the user's own dev machine per
AGENTS.md's "adapt to the real stack" principle -- this needs to run
anywhere Python 3 already runs, no extra install step.
"""
import json
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
MAIN_GO = REPO_ROOT / "backend" / "cmd" / "api" / "main.go"
OUT_PATH = REPO_ROOT / "backend" / "internal" / "apidocs" / "openapi.json"

ROUTE_RE = re.compile(
    r'mux\.(?:Handle|HandleFunc)\(\s*"(GET|POST|PUT|DELETE|PATCH) ([^"]+)"'
)

PUBLIC_ROUTES = {
    ("GET", "/api/v1/health"),
    ("GET", "/api/v1/ready"),
}


def go_path_to_openapi(path: str) -> str:
    # net/http 1.22+ wildcards like {id} are already OpenAPI-compatible.
    # A trailing "/" prefix-match route (e.g. "/api/v1/devices/") becomes a
    # documented {rest} wildcard tail, since it really does match arbitrary
    # sub-paths at runtime.
    if path.endswith("/") and path.count("{") == 0:
        return path + "{rest}"
    return path


def tag_for(path: str) -> str:
    parts = [p for p in path.split("/") if p]
    for p in parts:
        if p in ("api", "v1"):
            continue
        return p
    return "root"


def main() -> int:
    text = MAIN_GO.read_text()
    routes = {}
    for m in ROUTE_RE.finditer(text):
        method, raw_path = m.group(1), m.group(2)
        oas_path = go_path_to_openapi(raw_path)
        routes.setdefault(oas_path, {})[method.lower()] = (method, raw_path)

    paths = {}
    for oas_path in sorted(routes.keys()):
        item = {}
        for http_method, (method, raw_path) in sorted(routes[oas_path].items()):
            is_public = (method, raw_path) in PUBLIC_ROUTES
            op = {
                "summary": f"{method} {raw_path}",
                "tags": [tag_for(raw_path)],
                "responses": {
                    "200": {"description": "OK"},
                    "401": {"description": "Unauthorized"},
                },
            }
            if not is_public:
                op["security"] = [{"sessionCookie": []}, {"apiKey": []}]
            else:
                op["security"] = []
            item[http_method] = op
        paths[oas_path] = item

    spec = {
        "openapi": "3.0.3",
        "info": {
            "title": "RoutingNMS API",
            "version": "0.4.0",
            "description": (
                "Auto-generated from backend/cmd/api/main.go's route table "
                "by backend/scripts/gen_openapi.py (Feature 0.4). Path and "
                "method coverage is complete and regenerated from source; "
                "request/response body schemas are being filled in "
                "incrementally per endpoint (0.4b) and are not yet present "
                "for most routes."
            ),
        },
        "servers": [{"url": "/"}],
        "components": {
            "securitySchemes": {
                "sessionCookie": {
                    "type": "apiKey",
                    "in": "cookie",
                    "name": "routingnms_session",
                },
                "apiKey": {
                    "type": "apiKey",
                    "in": "header",
                    "name": "X-API-Key",
                },
            }
        },
        "paths": paths,
    }

    OUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    OUT_PATH.write_text(json.dumps(spec, indent=2, sort_keys=False) + "\n")
    print(f"Wrote {OUT_PATH} with {len(paths)} paths ({sum(len(v) for v in routes.values())} operations)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
