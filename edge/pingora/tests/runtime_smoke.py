"""Exercise the Edge public HTTP and enrollment interfaces against a local controller."""

import base64
import hashlib
import http.client
import http.server
import io
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import struct
import re
import subprocess
import sys
import tempfile
import threading
import time
import urllib.parse
import zipfile


def wait_for(check, *, seconds=12):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            if result := check():
                return result
        except (OSError, http.client.HTTPException, subprocess.CalledProcessError):
            pass
        time.sleep(0.1)
    raise AssertionError(f"timed out waiting for {check.__name__}")


def unused_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


class Controller(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def answer(self, body, status=200, headers=()):
        self.send_response(status)
        self.send_header("Content-Length", str(len(body)))
        for name, value in headers:
            self.send_header(name, value)
        self.end_headers()
        self.wfile.write(body)

    def data(self, data, status=200):
        self.answer(json.dumps({"error_msg": "", "data": data}).encode(), status)

    def manifest(self):
        if self.server.active is None:
            return {}
        self.server.config_bodies.clear()
        active = self.server.active
        meta = {key: value for key, value in active.items() if key != "payload_json"}
        self.server.config_bodies["sites.v1"] = active["payload_json"]
        resources = {"sites.v1": {"sha256": active["payload_checksum"], "metadata": meta}}
        def add(key, value, revision=None, inline=True):
            body = json.dumps(value, separators=(",", ":"))
            resources[key] = {"sha256": hashlib.sha256(body.encode()).hexdigest()}
            if inline:
                resources[key]["inline_json"] = body
            if revision:
                resources[key]["revision"] = revision
            self.server.config_bodies[key] = body
        add("performance.v1", {"settings": self.server.performance})
        for group in self.server.ip_groups:
            add(f"waf.ip-group.v1/{group['id']}", group, group["checksum"], inline=False)
        return resources

    def do_POST(self):
        payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        if self.path.endswith("/register"):
            assert payload["engine"] == "pingora"
            assert payload["contract_version"] == 3
            assert payload["config_sync_version"] == 1
            if self.headers["X-Agent-Token"] == "discovery-credential":
                assert payload["registration_key"]
            else:
                assert self.headers["X-Agent-Token"] == "node-credential"
                assert "registration_key" not in payload
                self.server.registered_with_node_token = True
            self.data({"node_id": "fixture", "agent_token": "node-credential"})
        elif self.headers["X-Agent-Token"] != "node-credential":
            self.data(None, 401)
        elif self.path.endswith("/heartbeat"):
            self.server.heartbeats.append(payload)
            self.data({"config": {"version": 1, "resources": self.manifest()},
                       "agent_settings": {"heartbeat_interval": self.server.heartbeat_interval}})
        elif self.path.endswith("/apply-logs"):
            self.server.applies.append(payload)
            self.data({"id": len(self.server.applies)})

    def do_GET(self):
        if self.path.startswith("/api/v1/agent/config/pull?"):
            assert self.headers["X-Agent-Token"] == "node-credential"
            query = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            key = query["key"][0]
            self.server.pulls.append(key)
            self.answer(self.server.config_bodies[key].encode(),
                        headers=(("Content-Type", "application/json"),))
        elif self.path == "/api/v1/agent/pages/projects/10/latest/hash":
            package, metadata = self.server.pages
            self.data(metadata)
        elif self.path == "/api/v1/agent/pages/projects/10/latest/package":
            package, metadata = self.server.pages
            self.answer(package)
        elif self.path == "/api/v1/agent/ws":
            self.protocol_version = "HTTP/1.1"
            key = (self.headers["Sec-WebSocket-Key"] + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode()
            self.send_response(101)
            self.send_header("Upgrade", "websocket")
            self.send_header("Connection", "Upgrade")
            self.send_header("Sec-WebSocket-Accept", base64.b64encode(hashlib.sha1(key).digest()).decode())
            self.end_headers()
            self.server.config_ws = self.connection
            self.connection.settimeout(20)
            try:
                while self.connection.recv(4096):
                    pass
            except OSError:
                pass
            finally:
                if self.server.config_ws is self.connection:
                    self.server.config_ws = None
        elif self.path == "/origin-error":
            self.answer(b"original", 503)
        elif self.path == "/slow-origin":
            time.sleep(1.3)
            try:
                self.answer(b"slow origin")
            except OSError:
                pass
        else:
            self.server.origins[self.path] = self.server.origins.get(self.path, 0) + 1
            self.answer(f"{self.headers['Host']} {self.path}".encode(),
                        headers=(("Content-Type", "text/plain"),))


def route(origin, rid, host, **options):
    value = dict(id=rid, site_name=host, domains=[host], enabled=True,
                 origin_url=origin, upstreams=[origin], domain_cert_ids=[1],
                 enable_https=True, enable_http2=True, redirect_http=True)
    value.update(options)
    return value


def release(seq, origin, certificate, key):
    shared = {
        "routes": [
            route(origin, 1, "edge.test", origin_host="origin.test"),
            route(origin, 2, "h1.edge.test", enable_http2=False),
            route(origin, 3, "login.edge.test", basic_auth_username="alice",
                  basic_auth_password="fixture-password", site_auth_ttl_hours=1,
                  site_auth_key="a" * 64),
            route(origin, 4, "pow.edge.test", pow_enabled=True,
                  pow_config={"difficulty": 1, "algorithm": "fast", "session_ttl": 600,
                              "challenge_ttl": 300, "whitelist": {}, "blacklist": {}}),
            route(origin, 5, "pages.edge.test", upstream_type="pages", basic_auth_enabled=True,
                  basic_auth_username="alice", basic_auth_password="fixture-password",
                  pages_deployment={"project_id": 10, "entry_file": "index.html",
                                    "api_proxy_enabled": True, "api_proxy_path": "/api",
                                    "api_proxy_pass": origin, "api_proxy_rewrite": "/v1"}),
            route(origin, 6, "cache.edge.test", cache_enabled=True, cache_policy="all"),
            route(origin, 7, "waf.edge.test"),
        ],
        "edge_config": {
            "origin_error_page_enabled": True,
            "origin_error_page_status_codes": ["503"],
            "trusted_proxy_cidrs": [],
            "site_auth_html": "<form>fixture sign-in {{login_error}}</form>",
        },
        "waf": {
            "rule_groups": [{"id": 1, "enabled": True, "is_global": False,
                             "graph": {"entry": "ip", "nodes": {
                                 "ip": {"type": "ip_match", "config": {"ip_group_ids": [33]},
                                        "next": {"true": "block", "false": "allow"}},
                                 "block": {"type": "block", "config": {"status_code": 403}},
                                 "allow": {"type": "allow"},
                             }}}],
            "ip_groups": [{"id": 33, "enabled": True, "ip_list": ["127.0.0.1"]}],
            "bindings": [{"route_id": 7, "rule_group_ids": [1]}],
        },
    }
    payload = json.dumps({
        "schema_version": 1,
        "contract_version": 3,
        "required_capabilities": ["tls-sni-h2-v1", "policy-runtime-v1", "shared-snapshot-v1"],
        "snapshot": shared,
        "assets": {
            "certificates": {"1": {"cert_pem": certificate, "key_pem": key}},
            "origin_error_html": "<h1>{{host}} {{status}}</h1>",
        },
    })
    return {
        "engine": "pingora", "version": f"pingora-fixture-{seq}",
        "checksum": "fixture-checksum", "release_sequence": seq,
        "payload_json": payload,
        "payload_checksum": hashlib.sha256(payload.encode()).hexdigest(),
    }


def make_pages(deployment, contents):
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w") as archive:
        archive.writestr("index.html", contents)
    package = buffer.getvalue()
    metadata = {
        "project_id": 10, "deployment_id": deployment,
        "hash": hashlib.sha256(package).hexdigest(), "package_size": len(package),
        "file_count": 1, "total_size": len(contents),
    }
    return package, metadata


def browser_proof(boot):
    nonce = 0
    while True:
        digest = hashlib.sha256(f"{boot['randomData']}{nonce}".encode()).hexdigest()
        if digest.startswith("0" * boot["difficulty"]):
            return urllib.parse.urlencode({"id": boot["id"], "response": digest, "nonce": nonce})
        nonce += 1


def main(binary):
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Controller)
    server.heartbeats = []
    server.applies = []
    server.registered_with_node_token = False
    server.ip_groups = []
    server.config_bodies = {}
    server.pulls = []
    server.config_ws = None
    server.performance = None
    server.heartbeat_interval = 250
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    control_url = f"http://127.0.0.1:{server.server_port}"
    origin_server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Controller)
    origin_server.origins = {}
    threading.Thread(target=origin_server.serve_forever, daemon=True).start()
    origin = f"http://127.0.0.1:{origin_server.server_port}"
    http_port, tls_port = unused_port(), unused_port()
    while tls_port == http_port:
        tls_port = unused_port()

    def request(host, path="/", *, use_http2=False, headers=(), data=None):
        scheme = "https"
        curl = os.environ.get("CURL", shutil.which("curl"))
        args = [curl, "-kSs", "--max-time", "4", "--noproxy", "*",
                "--resolve", f"{host}:{tls_port}:127.0.0.1",
                "--http2" if use_http2 else "--http1.1",
                "-w", "\n%{http_code}\n%{http_version}", "-D", "-"]
        for header in headers:
            args.extend(["-H", header])
        if data is not None:
            args.extend(["--data", data])
        result = subprocess.run(args + [f"{scheme}://{host}:{tls_port}{path}"],
                                check=True, capture_output=True, text=True)
        head_body, status, version = result.stdout.rsplit("\n", 2)
        head, body = head_body.split("\n\n", 1)
        return int(status), version, head, body

    def wake():
        ws = wait_for(lambda: server.config_ws)
        payload = b'{"type":"config_changed"}'
        ws.sendall(struct.pack("!BB", 0x81, len(payload)) + payload)

    def http_route():
        conn = http.client.HTTPConnection("127.0.0.1", http_port, timeout=1)
        try:
            conn.request("GET", "/test?query=1", headers={"Host": "edge.test"})
            response = conn.getresponse()
            return response.status, response.getheader("Location")
        finally:
            conn.close()

    with tempfile.TemporaryDirectory(prefix="openflare-edge-smoke-") as root:
        path = Path(root)
        certificate, key = path / "cert.pem", path / "key.pem"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                        "-subj", "/CN=edge.test", "-days", "1", "-out", str(certificate),
                        "-keyout", str(key)], check=True, capture_output=True)
        server.pages = make_pages(1, b"first page")
        server.active = release(1, origin, certificate.read_text(), key.read_text())
        token = path / "bootstrap"
        token.write_text("discovery-credential")
        ready = path / "ready.json"
        args = [binary, "run", "--server-url", control_url, "--token-file", str(token),
                "--state-dir", root, "--http-listen", f"127.0.0.1:{http_port}",
                "--tls-listen", f"127.0.0.1:{tls_port}"]

        with (path / "edge.log").open("w+") as log:
            def launch(*options, env=None):
                return subprocess.Popen(args + list(options), env={**os.environ, **(env or {}), "RUST_LOG": "info"},
                                        stdout=log, stderr=subprocess.STDOUT)

            notify = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
            if sys.platform == "linux":
                address = str(path / "notify.sock")
                notify.bind(address)
                notify.settimeout(12)
                edge = launch(env={**os.environ, "NOTIFY_SOCKET": address})
                try:
                    assert notify.recv(4096).startswith(b"READY=1\n")
                except Exception:
                    edge.kill()
                    edge.wait(timeout=5)
                    notify.close()
                    log.seek(0)
                    sys.stderr.write(log.read())
                    raise
            else:
                edge = launch()
            try:
                wait_for(lambda: ready.exists())
                wait_for(lambda: any(item["result"] == "success" for item in server.applies))
                assert json.loads(ready.read_text())["applied_version"] == server.active["version"]
                assert json.loads((path / "identity.json").read_text())["agent_token"] == "node-credential"
                assert http_route() == (301, "https://edge.test/test?query=1")
                assert request("edge.test", "/test?query=1", use_http2=True)[::3] == (
                    200, "origin.test /test?query=1")
                assert request("edge.test", use_http2=True)[1] == "2"
                assert request("h1.edge.test", use_http2=True)[1].startswith("1")
                assert request("edge.test", "/origin-error")[::3] == (503, "<h1>edge.test 503</h1>")
                assert request("login.edge.test", "/private")[0] == 303
                assert "fixture sign-in" in request("login.edge.test", "/.openflare/site-auth")[3]
                result = request("login.edge.test", "/.openflare/site-auth?next=%2Fprivate",
                                 data="username=alice&password=fixture-password",
                                 headers=(f"Origin: https://login.edge.test:{tls_port}",))
                assert result[0] == 303 and "__Host-openflare_site_auth=" in result[2]
                challenge = request("pow.edge.test")
                boot = json.loads(re.search(r'id="openflare-pow-config" type="application/json">(.*?)</script>', challenge[3]).group(1))
                proof = request("pow.edge.test", "/.within.website/x/cmd/anubis/api/pass-challenge?" + browser_proof(boot))
                assert proof[0] == 200 and json.loads(proof[3]) == {"ok": True}
                assert "__openflare_pow=" in proof[2]
                assert request("pages.edge.test")[0] == 401
                basic = ("Authorization: Basic YWxpY2U6Zml4dHVyZS1wYXNzd29yZA==",)
                assert request("pages.edge.test", headers=basic)[3] == "first page"
                assert request("pages.edge.test", headers=basic + ("Range: bytes=0-4",))[::3] == (206, "first")
                assert request("pages.edge.test", "/api/users?page=1")[3].endswith(" /v1/users?page=1")
                server.pages = make_pages(2, b"second page")
                wait_for(lambda: request("pages.edge.test", headers=basic)[3] == "second page")
                assert request("waf.edge.test")[0] == 403
                server.ip_groups = [{"id": 33, "enabled": False, "ip_list": [], "checksum": "updated"}]
                wait_for(lambda: request("waf.edge.test")[0] == 200)
                assert request("cache.edge.test", "/cache-test")[0] == 200
                assert request("cache.edge.test", "/cache-test")[0] == 200
                assert origin_server.origins["/cache-test"] == 1
                wait_for(lambda: any(h.get("current_version") == server.active["version"]
                                     and "profile" in h and "host_metrics" in h and "edge_health" in h
                                     for h in server.heartbeats))

                assert "sites.v1" in server.pulls
                assert "waf.ip-group.v1/33" in server.pulls
                assert server.pulls.count("sites.v1") == 1
                policy = json.loads(Path(__file__).with_name("performance_policy.json").read_text())
                policy["threads"] = 2
                server.performance = policy
                server.heartbeat_interval = 10_000
                wake()

                def has_worker_threads(expected):
                    content = (path / "edge.log").read_text()
                    return f"threads={expected}" in re.sub(r"\x1b\[[0-9;]*m", "", content)

                assert wait_for(lambda: has_worker_threads(2), seconds=8)
                assert edge.poll() is None
                wait_for(lambda: http_route()[0] == 301)
                state_file = path / "performance-policy.json"
                assert json.loads(state_file.read_text())["settings"]["threads"] == 2
                assert request("edge.test", "/slow-origin")[0] == 200

                # A peer-only change takes effect without restarting the worker runtime.
                server.performance = {**policy, "read_timeout": 1}
                wake()
                wait_for(lambda: json.loads(state_file.read_text())["settings"]["read_timeout"] == 1, seconds=3)
                assert request("edge.test", "/slow-origin")[0] != 200
                assert (path / "edge.log").read_text().count("starting Pingora workers") == 2
                assert "performance.v1" not in server.pulls

                server.heartbeat_interval = 250
                server.active = release(2, origin, certificate.read_text(), key.read_text())
                server.active["payload_checksum"] = "invalid candidate"
                wake()
                wait_for(lambda: any(log["release_sequence"] == 2 and log["result"] == "failed"
                                     for log in server.applies))
                assert request("edge.test", use_http2=True)[0] == 200
                assert json.loads((path / "snapshot.json").read_text())["release_sequence"] == 1

                edge.kill()
                edge.wait(timeout=5)
                edge = launch()
                wait_for(lambda: server.registered_with_node_token)
                wait_for(lambda: request("edge.test", use_http2=True)[0] == 200)

                edge.kill()
                edge.wait(timeout=5)
                server.registered_with_node_token = False
                token.write_text("node-credential")
                edge = launch("--token-type", "node")
                wait_for(lambda: server.registered_with_node_token)

                edge.kill()
                edge.wait(timeout=5)
                token.unlink()
                ready.unlink()
                server.shutdown()
                edge = launch()
                wait_for(lambda: ready.exists())
                assert http_route() == (301, "https://edge.test/test?query=1")
                assert request("edge.test", use_http2=True)[0] == 200
                if sys.platform == "linux":
                    previous = edge
                    replacement = launch("--upgrade")
                    try:
                        wait_for(lambda: (path / "openflare-edge-upgrade.sock").exists())
                        previous.send_signal(signal.SIGQUIT)
                        wait_for(lambda: json.loads(ready.read_text())["pid"] == replacement.pid)
                        edge = replacement
                        assert http_route() == (301, "https://edge.test/test?query=1")
                        assert request("edge.test", use_http2=True)[0] == 200
                    finally:
                        for candidate in (previous, replacement):
                            if candidate is not edge and candidate.poll() is None:
                                candidate.kill()
                                candidate.wait(timeout=5)

            except Exception:
                log.seek(0)
                sys.stderr.write(log.read())
                raise
            finally:
                if edge.poll() is None:
                    edge.kill()
                    edge.wait(timeout=5)
                notify.close()
                server.server_close()
                origin_server.shutdown()
                origin_server.server_close()
    print("Edge runtime smoke passed")


if __name__ == "__main__":
    main(str(Path(sys.argv[1]).resolve()))
