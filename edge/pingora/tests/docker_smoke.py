"""Verify Edge enrollment, proxying and state recovery through the Docker entrypoint."""

import http.server
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import uuid

from runtime_smoke import Controller, make_pages, release, unused_port, wait_for


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True, timeout=45).strip()


def main(image):
    binary_version = docker("run", "--rm", image, "--version").split()[1]
    with tempfile.TemporaryDirectory() as tmp, http.server.ThreadingHTTPServer(
        ("127.0.0.1", 0), Controller
    ) as controller:
        cert, key = Path(tmp) / "cert.pem", Path(tmp) / "key.pem"
        subprocess.run(
            ["openssl", "req", "-x509", "-nodes", "-newkey", "rsa:2048", "-days", "1",
             "-subj", "/CN=edge.test", "-addext", "subjectAltName=DNS:edge.test",
             "-out", str(cert), "-keyout", str(key)],
            check=True, capture_output=True,
        )
        server_url = f"http://127.0.0.1:{controller.server_port}"
        controller.active = release(1, server_url, cert.read_text(), key.read_text())
        controller.binary_version = binary_version
        controller.heartbeat_interval = 250
        controller.heartbeats = []
        controller.applies = []
        controller.pulls = []
        controller.config_bodies = {}
        controller.ip_groups = []
        controller.performance = None
        controller.config_ws = None
        controller.site_pull_conflict = False
        controller.pages = make_pages(1, b"fixture")
        controller.origins = {}
        controller.registered_with_node_token = False
        threading.Thread(target=controller.serve_forever, daemon=True).start()

        ports = [unused_port(), unused_port()]
        while ports[0] == ports[1]:
            ports[1] = unused_port()
        volume = docker("volume", "create", f"openflare-edge-smoke-{uuid.uuid4().hex}")
        settings = [
            "--network", "host",
            "-v", f"{volume}:/var/lib/openflare-edge",
            "-e", f"OPENFLARE_EDGE_SERVER_URL={server_url}",
            "-e", f"OPENFLARE_EDGE_HTTP_LISTEN=127.0.0.1:{ports[0]}",
            "-e", f"OPENFLARE_EDGE_TLS_LISTEN=127.0.0.1:{ports[1]}",
        ]
        container = None
        try:
            container = docker(
                "run", "-d", *settings, "-e", "OPENFLARE_EDGE_TOKEN=discovery-credential", image
            )
            wait_for(lambda: docker("inspect", "-f", "{{.State.Health.Status}}", container) == "healthy",
                     seconds=45)
            assert docker("exec", container, "id", "-u") == "10001"
            assert any(item["current_version"] == controller.active["version"]
                       for item in controller.heartbeats)
            assert subprocess.check_output(
                ["curl", "--fail", "--silent", "--max-time", "5", "--noproxy", "*",
                 "--cacert", str(cert), "--resolve", f"edge.test:{ports[1]}:127.0.0.1",
                 f"https://edge.test:{ports[1]}/docker-smoke"], text=True,
            ) == "origin.test /docker-smoke"

            docker("stop", "--time", "10", container)
            docker("rm", container)
            container = docker("run", "-d", *settings, image)
            wait_for(lambda: controller.registered_with_node_token, seconds=30)
            wait_for(lambda: docker("inspect", "-f", "{{.State.Health.Status}}", container) == "healthy",
                     seconds=45)
        except Exception:
            if container:
                print(docker("logs", container), file=sys.stderr)
            raise
        finally:
            if container:
                docker("rm", "-f", container)
            docker("volume", "rm", volume)
            controller.shutdown()
    print("Edge Docker smoke passed")


if __name__ == "__main__":
    main(sys.argv[1])
