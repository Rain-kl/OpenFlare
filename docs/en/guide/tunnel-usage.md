# Tunnels and Intranet Penetration

Expose private L7 web apps and L4 TCP/UDP services across OpenFlare edge nodes through **Relay Pools → Public Relays (`openflare-relay`) → Tunnels & Connectors (`openflared`)**.

## 1. Configure a Relay Pool

Open **Tunnels** > **Relay Pools** to inspect the default pool or create one:

1. Set **Preferred Transport** (`auto` negotiates in order: `noise`, `websocket`, then `tcp`).
2. Set **HA Relay Count** (default `2`; deploy at least two relays across separate zones in production).
3. Configure **Allowed / Blocked Target CIDRs** (`169.254.169.254/32` is blocked by default; an empty allowlist permits all non-blocked targets).

## 2. Deploy Public Relays

1. Open **Tunnels** > **Relay PoPs**, add a relay or copy the discovery bootstrap command, and optionally set a **Failure-Domain Zone Tag** and **Co-located Edge Node ID**.
2. Start `openflare-relay` on a public host following [Deploy Relay](../deployment/relay.md) and confirm it reaches **Online**.

## 3. Create a Tunnel and Start Connectors

1. Open **Tunnels** > **Tunnels** > **Create Tunnel**, select the **Relay Pool**, transport preference, and tunnel-level **Allowed / Blocked Target CIDRs**.
2. Copy the `oftok1_` bootstrap token and launch `openflared` on your private host following [Deploy OpenFlared](../deployment/openflared.md).
3. Verify the connector is **Online** with active relay attachments in the tunnel detail view.

## 4. Expose Services

- **L7 HTTP / WebSocket sites**: Under **Proxy Routes**, set origin mode to **Intranet Tunnel**, select the tunnel, and enter the internal upstream URL (such as `http://127.0.0.1:8080`; enable `insecure_skip_verify` for self-signed internal HTTPS).
- **L4 TCP / UDP services**: On the tunnel detail page, click **Add L4 Service**, select protocol, target relay, public listen port (avoid `7443`, `7444`, `8443`, and ports in use), and private target address (such as `127.0.0.1:22`), then expose via `<RELAY_PUBLIC_IP>:<LISTEN_PORT>`.

## Operations & Troubleshooting

| Action / Symptom | Guidance |
| :--- | :--- |
| Drain relay | Click **Drain** on a relay detail page to stop new assignments and let active `openflared` sessions migrate before maintenance |
| Active-active HA | Run multiple `openflared` instances with the same tunnel token for automatic failover |
| Quarantine & rotate | Click **Quarantine** on a tunnel or relay to revoke access immediately; **Release Quarantine** issues a fresh token |
| Connector offline | Verify outbound connectivity to the Server URL and relay ports `7443`, `7444`, and `8443`, and ensure a healthy relay exists; after rotation, update `/var/lib/openflared/bootstrap-token` and restart |
| HTTP `403` / `502` | Check pool and tunnel CIDR rules for `403`; verify target reachability from `openflared` or enable `insecure_skip_verify` for `502` |
| L4 service bind failure | Avoid `7443`, `7444`, `8443`, and conflicting L4 ports, and open the listen port in host and cloud firewalls |

See [Tunnel Architecture Design](../design/tunnel-design.md) for protocol internals.
