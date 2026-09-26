// Package apidocs serves the RoutingNMS OpenAPI spec (Feature 0.4: API-First
// Discipline) and a Swagger UI page to browse it.
//
// openapi.json is generated from the real route table in cmd/api/main.go by
// backend/scripts/gen_openapi.py -- see that script's header comment for the
// scope decision (path/method coverage is complete; request/response body
// schemas are filled in incrementally as a follow-up, 0.4b). It is embedded
// at build time via go:embed rather than read from disk at runtime, so the
// binary is self-contained the same way every other static asset in this
// project is.
package apidocs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openAPISpec []byte

// SpecHandler serves the raw OpenAPI document.
func SpecHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(openAPISpec)
}

// UIHandler serves a minimal Swagger UI page pointed at SpecHandler's route.
// Swagger UI's JS/CSS are loaded from a CDN rather than vendored, consistent
// with how this project already loads a handful of frontend assets -- this
// is an internal engineering tool page (not shipped to NOC end users), so it
// doesn't need the offline guarantees the main frontend build has.
func UIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(swaggerUIPage))
}

const swaggerUIPage = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>RoutingNMS API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body style="margin:0">
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "/api/v1/openapi.json",
        dom_id: "#swagger-ui",
        presets: [SwaggerUIBundle.presets.apis],
      });
    };
  </script>
</body>
</html>`
