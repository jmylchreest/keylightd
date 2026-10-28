"""Read-only, container-local data for deterministic UI captures."""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

LIGHTS = {
    "key": {"id": "key", "name": "Key Light", "on": True, "brightness": 65, "temperature": 200},
    "fill": {"id": "fill", "name": "Fill Light", "on": False, "brightness": 35, "temperature": 250},
}
GROUPS = [{"id": "studio", "name": "Studio", "lights": ["key", "fill"]}]


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        routes = {"/api/v1/lights": LIGHTS, "/api/v1/groups": GROUPS}
        routes.update({f"/api/v1/lights/{key}": value for key, value in LIGHTS.items()})
        routes["/api/v1/groups/studio"] = GROUPS[0]
        if self.path not in routes:
            self.send_error(404)
            return
        body = json.dumps(routes[self.path]).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


HTTPServer(("127.0.0.1", 9999), Handler).serve_forever()
