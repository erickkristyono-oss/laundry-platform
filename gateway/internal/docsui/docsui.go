// Package docsui serves the hand-authored OpenAPI spec (docs/openapi.yaml,
// generated from docs/07-api-contract.md — that Markdown doc remains the
// source of truth) plus a Swagger UI page to browse/try it, and to make it
// importable into Postman via a plain URL. Public, unauthenticated: it is
// documentation, not an API surface, so it is mounted directly on the
// Gateway's mux rather than going through the proxy/auth chain.
package docsui

import (
	"net/http"
	"os"
)

// OpenAPISpec serves the raw spec file from specPath (see config.OpenAPISpecPath —
// baked into the Gateway image at /openapi.yaml by Dockerfile.gateway).
func OpenAPISpec(specPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(specPath)
		if err != nil {
			http.Error(w, "openapi spec not available", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*") // Postman/Swagger Editor may fetch this cross-origin
		_, _ = w.Write(data)
	}
}

// SwaggerUI serves a static page that loads swagger-ui-dist from a CDN and
// points it at /docs/openapi.yaml. No build step, no vendored assets.
func SwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerHTML))
}

const swaggerHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>LaundryKu API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
  <style>body { margin: 0; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "/docs/openapi.yaml",
        dom_id: "#swagger-ui",
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout",
      });
    };
  </script>
</body>
</html>`
