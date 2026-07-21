#!/usr/bin/env python3
"""Minimal mock of the three device endpoints (time/activate/report), used
only for a manual smoke test of the simulator binary's HTTP behavior and
main loop -- it does not verify HMAC signatures the way the real Go server
does (that's already covered by test_protocol's RFC-vector-checked unit
tests). Not part of the build; not linked from CMakeLists.txt/Makefile.
"""
import hashlib
import json
import time
import http.server

NONCE = "abc123nonce"
TOKEN = "tok-" + "0" * 60
sent_cmd = {"set_cfg": False, "set_probe": False, "ota": False}

FIRMWARE = (b"UBIBOT-FAKE-FIRMWARE-" * 500)  # ~10.5KB, big enough to see chunked progress
FIRMWARE_SHA256 = hashlib.sha256(FIRMWARE).hexdigest()


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
        body = self._read_body()
        now = int(time.time())
        if self.path == "/api/v1/auth/time":
            self._reply({"c": 0, "t": now, "n": NONCE})
        elif self.path == "/api/v1/auth/activate":
            self._reply({"c": 0, "token": TOKEN, "exp": 3600})
        elif self.path == "/api/v1/data/report":
            req = json.loads(body)
            print("REPORT recs=%d ack=%s nak=%s prb=%s ota=%s" % (
                len(req.get("recs", [])), req.get("ack"), req.get("nak"),
                req.get("prb"), req.get("ota")))
            resp = {"c": 0, "t": now}
            cmd = []
            if not sent_cmd["set_cfg"]:
                cmd.append({"id": "c-cfg-1", "tp": "set_cfg", "a": {"ci": 5, "ui": 8}})
                sent_cmd["set_cfg"] = True
            elif not sent_cmd["set_probe"]:
                cmd.append({"id": "c-probe-1", "tp": "set_probe",
                            "a": {"op": "upsert", "pid": "p1", "key": "soil_temp",
                                  "iface": "rs485", "proto": "modbus", "scale": 0.1, "offset": 0}})
                sent_cmd["set_probe"] = True
            elif not sent_cmd["ota"]:
                cmd.append({"id": "c-ota-1", "tp": "ota",
                            "a": {"action": "start", "version": "1.0.1", "url": "/fw/1.0.1.bin",
                                  "size": len(FIRMWARE), "sha256": FIRMWARE_SHA256}})
                sent_cmd["ota"] = True
            if cmd:
                resp["cmd"] = cmd
            self._reply(resp)
        else:
            self._reply({"c": 5000, "m": "not found"}, 404)

    def do_GET(self):
        if self.path == "/fw/1.0.1.bin":
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("Content-Length", str(len(FIRMWARE)))
            self.end_headers()
            self.wfile.write(FIRMWARE)
            print("FIRMWARE served, %d bytes, sha256=%s" % (len(FIRMWARE), FIRMWARE_SHA256))
        else:
            self._reply({"c": 5000, "m": "not found"}, 404)

    def log_message(self, fmt, *args):
        pass


if __name__ == "__main__":
    http.server.HTTPServer(("127.0.0.1", 8099), Handler).serve_forever()
