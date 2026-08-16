# Tunnel & Intranet Penetration

You will learn: the design principles of OpenFlare's intranet penetration tunnels, core concepts (relay nodes and tunnel clients), and how to publish an intranet dev environment or private cloud service to a public domain step by step, securely and stably.

In many real development and ops scenarios, origin services run inside a LAN, on a local dev machine, or in a private VPC — with no public IP and no way to configure port mapping on the border firewall or router.

OpenFlare provides a complete **reverse-relay tunnel penetration** solution. You only initiate an outbound secure connection from the intranet to a public relay node — no inbound ports need to be configured — and public web traffic is routed into the intranet origin, while enjoying the gateway's automatic TLS certificate management and WAF protection.

---

## Core Concepts

Before using intranet penetration, get familiar with these components:

| Component | Description | Corresponding Entity |
| --- | --- | --- |
| **Relay node** | a traffic relay service deployed at the public edge; listens for the intranet client's long connections and bridges gateway Agent (OpenResty) and intranet traffic | `tunnel_relay` node guarded by `openflare-relay` |
| **Tunnel** | a logical penetration client instance with a globally unique ID and an auth token, identifying one concrete intranet environment | `tunnel_client` node created in「Node Management」, assigned a dedicated Tunnel Token |
| **Tunnel client** | a lightweight controller running in the intranet; auto-manages the underlying frpc tunnel child processes based on Server-dispatched config | `openflared` container or standalone binary deployed in the intranet |
| **Tunnel upstream** | a special reverse proxy type in route rules. With this type, the gateway forwards public traffic to the local relay's Vhost port, eventually reaching the intranet origin | reverse proxy type configured in the「Rule Management」detail page, origin mode「Intranet Tunnel」with a bound Tunnel node |

---

## Recommended Order

To publish an intranet service to the public, follow this order:

1. Register and deploy at least one public **Relay node** and keep it online.
2. Go to **「Node Management」**, create a node of type **Tunnel node (tunnel_client)**, and get the dedicated Token.
3. Deploy and start the **tunnel client (OpenFlared)** on the intranet server.
4. Confirm the Tunnel node's status shows「Online」in the admin panel.
5. Add or edit a rule in **「Rule Management」**; in the「Reverse Proxy」tab choose origin mode **「Intranet Tunnel」**, bind the Tunnel node, and enter the intranet service port (e.g. `127.0.0.1:8080`).
6. Publish and activate the new version.
7. Access via the public domain to verify the tunnel link.

---

## Detailed Steps

### Step 1: Prepare a Relay Node

Intranet traffic needs a public relay node to transit. Before starting, make sure you have a usable relay server on the public network.

1. Log in to the admin panel, go to **「Node Management」**.
2. Add a new node and set **Node Type** to **Relay node (tunnel_relay)**.
3. After saving, copy the node's dedicated `agent_token`.
4. Start `openflare-relay` on your public server. Docker quick run:

   ```bash
   docker run -d --name openflare-relay --restart unless-stopped \
     -p 7000:7000 \
     -e OPENFLARE_SERVER_URL=http://<your-Server-public-IP>:3000 \
     -e OPENFLARE_AGENT_TOKEN=<the-AgentToken-you-copied> \
     -v openflare-relay-data:/var/lib/openflare-relay \
     ghcr.io/rain-kl/openflare-relay:latest
   ```

   > [!IMPORTANT]
   > Open port `7000` (the frpc client connection control port, default `relay_bind_port`) in the cloud server's security group. If your Server and relay node are on the same machine, `OPENFLARE_SERVER_URL` here should point to the Server's public or intranet communication IP.

### Step 2: Create a Tunnel Node in the Admin Panel

1. Navigate to **「Node Management」** in the admin sidebar.
2. Click **「Add Node」**; in the dialog choose node type **「Tunnel node (tunnel_client)」**.
3. Fill in the node name and description, click save.
4. Click into the Tunnel node's detail page; find the dedicated **Tunnel Token** and the one-click client deployment command.

### Step 3: Deploy the Intranet Client (OpenFlared)

Back on your intranet server, run the client with the copied deployment command.

#### Option A: Deploy with Docker (recommended)

The official `openflared` image bundles the supervisor daemon and the `frpc` runtime — out of the box, no extra dependencies:

```bash
docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_SERVER_URL=http://<your-Server-public-IP>:3000 \
  -e OPENFLARE_TUNNEL_TOKEN=<the-TunnelToken-you-copied> \
  -v openflared-data:/app/data \
  ghcr.io/rain-kl/openflared:latest
```

#### Option B: Run the host binary manually

If Docker isn't convenient, download or build the `flared` binary yourself:

1. Create a `flared.json` config file next to the program on the intranet machine:
   ```json
   {
     "server_url": "http://<your-Server-public-IP>:3000",
     "tunnel_token": "<the-TunnelToken-you-copied>",
     "frpc_path": "/usr/local/bin/frpc",
     "data_dir": "./data"
   }
   ```
2. Start it:
   ```bash
   ./flared -config ./flared.json
   ```

#### Status Confirmation

After starting, the intranet client sends heartbeat syncs to the control plane over outbound connections:
1. Refresh the **「Node Management」** list; the Tunnel node's status light should turn green **「Online」**.
2. Click into the node detail page to see which public Relay nodes the intranet client is connected to.

### Step 4: Configure the Route and Bind the Tunnel Upstream

Now configure public reverse proxying and domain access for your intranet service:

1. First go to **「Website Management」->「Domain List」** and register the domain you want to expose.
2. Go to **「Rule Management」**, click **「New Rule」** or edit an existing rule.
3. In the **「Reverse Proxy」** tab, switch the **origin mode** to **「Intranet Tunnel」**.
4. Select the online **Tunnel node** from the dropdown.
5. Fill in the **intranet target address** (a local address/port reachable by the intranet client, e.g. `127.0.0.1:8080`) and **intranet protocol** (usually `http`).
6. Configure other regular site options and save.

### Step 5: Publish and Apply

To let the gateway's OpenResty match and route domain traffic, publish a new config version:

1. Click **「Preview and Publish」** in the top-right nav; confirm the generated site config is correct.
2. In the dialog, click **「Confirm Publish」**.
3. The public-edge Agent now pulls the latest route: it forwards requests to the same-host `openflare-relay (frps)` vhost port.
4. The intranet client `openflared (frpc)` receives the relayed packets and safely forwards them to the intranet `127.0.0.1:8080` service, returning the response along the same path.
5. Visit the domain in a public browser to confirm the intranet service displays.

---

## Advanced Scenarios

### 1. One Tunnel, Multiple Services (multi-port mapping)

You don't need a separate `openflared` container for every intranet service.

To map multiple services in one intranet environment (e.g. `127.0.0.1:80` blog, `127.0.0.1:8080` API, `192.168.1.120:9000` intranet drive):
1. Keep this one `openflared` client online.
2. Create three separate website configs in the admin panel (each bound to its own public domain).
3. Select **the same tunnel** as the origin mode for all three.
4. Fill in the corresponding different ports or LAN IPs in each intranet target address (e.g. `127.0.0.1:80`, `127.0.0.1:8080`, `192.168.1.120:9000`).
5. Publish and activate — one tunnel, many uses.

### 2. Gateway Security Features Stack Seamlessly

Because all public traffic first enters the public Agent node — HTTPS/TLS handshake and WAF engine interception happen there — then travels through the secure tunnel to the intranet:

Your intranet service needs **zero modification** to enjoy:
* **One-click HTTPS**: select or apply an SSL certificate for the domain directly in the admin panel; data is encrypted end-to-end.
* **Global/custom WAF protection**: enable SQL injection blocking, XSS injection defense, and malicious geo-IP blocking.
* **Human challenge (CC PoW)**: one-click defense against malicious CC requests to intranet APIs.

---

## Troubleshooting

### 1. Tunnel shows「Offline」in the admin panel

* **Check the Token**: verify the `tunnel_token` in `flared` logs or env vars matches the one generated in the admin panel.
* **Check network connectivity**: the intranet server must be able to reach the Server address over outbound connections.
* **Relay firewall not open**: check that the relay node's public `7000` port (or custom `relay_bind_port`) is opened to the public in the security group.

### 2. Public domain returns 502 Bad Gateway / 504 Gateway Timeout

* **Intranet service not running**: confirm the service at the intranet target address is started and listening on the intranet server.
* **Target address unreachable**: if the intranet address is `127.0.0.1:8080`, ensure the service runs on the same host as `openflared`; if it's a LAN IP `192.168.x.x`, test LAN reachability from inside the `openflared` container.
* **Check node state and logs**: view the Tunnel node detail and「Apply Records」in the admin panel; frpc process errors are logged in detail in the `flared` logs on the intranet host.

### 3. Multi-relay network flapping or retry failures

* When the control plane is associated with multiple Relay nodes, `openflared` spawns a separate frpc supervisor per Relay and periodically pulls topology state from the control plane within the `sync_interval` configured in `flared.json` (default 30s).
* If a relay node frequently drops due to network jitter, the system auto-triggers exponential backoff retries (initial 1s, cap 60s). You may see `frpc process missing, starting` in the host logs — that's normal process self-healing; it reconnects automatically after the network recovers.
