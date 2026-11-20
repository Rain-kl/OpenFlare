# Deploy Pingora Edge

Create a node and copy its installation command from the console, or use a discovery token. A registered node can keep its node-specific token and record. Each edge host needs access to the controller over HTTPS.

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \\
  --server-url https://openflare.example.com \\
  --token-type node \\
  --token '<node token>'
```

For discovery enrollment, set `--token-type discovery`. The Linux installation package supports amd64 and arm64. The installer checks that the selected `edge-v*` release supports the current configuration sync protocol and waits for an active site snapshot. The service defaults to TCP ports 80 and 443. Run `bash install-openflare-edge.sh --help` for listener, version, and state directory options.

```bash
systemctl status openflare-edge
journalctl -u openflare-edge -f
```

Assign performance policies in the console. To remove the service, run `scripts/uninstall-openflare-edge.sh` from the release package on the edge host.
