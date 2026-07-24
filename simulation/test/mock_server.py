#!/usr/bin/env python3
"""Minimal mock of the two device endpoints (time/report), used only for a
manual smoke test of the simulator binary's HTTP behavior and main loop.
Not part of the build; not linked from CMakeLists.txt/Makefile.
"""
import json
import time
import http.server

DEVICES = {}  # sn -> {"pid":..., "disabled": bool}


class Handler(http.server.BaseHTTPRequestHandler):
    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        return self.rfile.read(length).decode("utf-8") if length else ""

    def _reply(self, obj, status=200):
        body = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        now = int(time.time())
        try:
            body = self._read_body()
            req = json.loads(body) if body else {}
        except (ValueError, UnicodeDecodeError):
            self._reply({"c": 1003, "m": "malformed body"}, 400)
            return

        if self.path == "/api/v1/auth/time":
            # No auth at all: this endpoint doesn't even look at pid/sn.
            self._reply({"c": 0, "t": now})
            return

        if self.path == "/api/v1/data/report":
            pid = req.get("pid")
            sn = req.get("sn")
            ts = req.get("ts")
            payloads = req.get("payloads")
            if not pid or not sn or ts is None or not isinstance(payloads, list):
                self._reply({"c": 1003, "m": "malformed body"}, 400)
                return

            if abs(now - ts) > 5 * 60:
                self._reply({"c": 1002, "m": "timestamp out of window"}, 400)
                return

            dev = DEVICES.setdefault(sn, {"pid": pid, "disabled": False})
            if dev["disabled"]:
                self._reply({"c": 1103, "m": "device disabled"}, 401)
                return

            print("REPORT pid=%s sn=%s payloads=%d" % (pid, sn, len(payloads)))
            for p in payloads:
                print("  ts=%s feed=%s" % (p.get("ts"), p.get("feed")))

            self._reply({"c": 0, "t": now})
            return

        self._reply({"c": 5000, "m": "not found"}, 404)

    def log_message(self, fmt, *args):
        pass


if __name__ == "__main__":
    http.server.HTTPServer(("127.0.0.1", 8099), Handler).serve_forever()
