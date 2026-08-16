# WAF Security Protection

You will learn: How the OpenFlare edge Web Application Firewall (WAF) works, its protection dimensions, how to manage and reference the three types of IP groups (Manual, Subscription, and Expr-based Automatic IP groups), configure CC protection challenges (PoW human-machine verification) and regional filtering, and achieve sub-second hot updates of IP group members without Nginx reloads.

---

## Core Concepts

Before configuring security policies, you need to understand the core components of the WAF:

| Concept | Description | Scope & Activation Method |
| --- | --- | --- |
| **WAF Rule Group (Rule Group)** | A logical collection of security rules, including: IP whitelists/blacklists (direct input or IP group references), country/region limits, CC protection (PoW), and custom block responses. | Supports global enablement or binding to single/multiple websites. **Modifying rule group definitions requires publishing and activating a configuration version**. |
| **IP Group (IP Group)** | A list container storing individual IPs or CIDR blocks. Divided into **Manual**, **Subscription**, and **Automatic** types. WAF rule groups reference IP groups by ID. | Belongs to dynamic resources. **IP group member updates support sub-second WebSocket hot-syncing, completely bypassing Nginx process reloads**. |
| **PoW Challenge (CC PoW)** | A human-machine verification challenge based on Proof of Work. By prompting browsers to solve hash collisions of a specified difficulty, it silently blocks malicious brute-force scripts and bots while keeping legitimate user experience smooth. | A configuration Tab in the rule group. **Modifying PoW parameters requires publishing and activating a configuration version**. |

---

## Recommended Configuration Sequence

When configuring security protections for your websites, we recommend doing so in the following order:

1. Navigate to IP Groups, creating the required **Manual IP Groups** (e.g., developer whitelist) or **Automatic IP Groups** (e.g., auto-blocked IPs based on 404 scans).
2. Create or edit a **WAF Rule Group**:
   * Bind the IP groups you want to reference or block.
   * Configure regional whitelists/blacklists for countries or provinces.
   * (Optional) Configure human-machine challenge parameters in the `PoW` Tab.
   * Set custom status codes (e.g., 403, 418) and HTML block pages in the `Block Response` Tab.
3. Associate the rule group with the corresponding **Website Configuration**.
4. Publish and activate the configuration version to let the edge node (Agent) apply the WAF rules to filter traffic.

---

## Detailed Step Guide

### Step 1: Manage and Configure IP Groups

IP groups are the foundations of large-scale IP filtering. OpenFlare provides three highly resilient types of IP groups:

#### 1. Manual IP Groups (Manual)
* **Purpose**: Statically maintain a list of verified trusted IPs or long-term blocked IPs/CIDR blocks.
* **Configuration**: Click "Create IP Group" -> select type "Manual" -> enter IPs or CIDRs line-by-line (e.g., `192.168.1.100` or `10.0.0.0/24`).

#### 2. Subscription IP Groups (Subscription)
* **Purpose**: Integrate third-party threat intelligence databases or IP ranges published by cloud providers.
* **Configuration**: Select type "Subscription" -> enter fetch URL (supports line-separated plain text or standard JSON formats). A background cron job on the Server periodically pulls the subscription source and updates the group members automatically.

#### 3. Automatic IP Groups (Automatic)
* **Purpose**: **The most aggressive automated defense channel against scans and brute-force attacks**.
* **Configuration**: Select type "Automatic" -> write Expr log aggregation logic. You can directly select built-in presets:
    * **Single IP High-Frequency 404 Scanning**: `request_count > 100 && status_404_ratio >= 0.8` (A single IP requesting over 100 times in the past hour with a 404 response ratio of at least 80%).
    * **Single IP Direct IP Access Mismatch**: `ip_host_count > 50 && ip_host_ratio > 0.5` (Bypassing domains to hit the server directly using IP address host headers).
* **Test & Run**: Click **"Test Rule"** before saving to preview IPs matching the current log window. Click **"Execute Now"** after saving to aggregate logs immediately and generate the block list.

> [!TIP]
> For the detailed syntax and available metrics of automatic IP groups, see [WAF Auto IP Group Expressions](./waf-ip-group-expr.md).

---

### Step 2: Create and Configure a WAF Rule Group

1. Navigate to the **"WAF"** section in the side menu, and click **"Create Rule Group"**.
2. Enter the rule group name (e.g., `production-api-shield`), and select if it is a "Global Rule Group".
3. Enter rule group details, and configure the tabs sequentially below:

#### 1. Whitelist / Blacklist Configuration (Allow / Block Lists)
* **Direct IPs**: Enter individual IPs or CIDR blocks line-by-line that need temporary whitelisting or blacklisting directly in the text area.
* **IP Group Reference**: Click "Bind IP Groups", selecting the manual, automatic, or subscription IP groups you configured in Step 1. Whitelists permit traffic instantly, whereas blacklists block it.

#### 2. Regional Restriction (GeoIP)
* **Description**: OpenFlare integrates GeoIP geolocation resolution.
* **Configuration**: Toggle the regional restriction switch, selecting "Allow Only" or "Block".
* * For example, if your service is only intended for domestic users, set the mode to "Allow Only" and check `China` in the country list.
* * Supports refining to specific provinces/regions, enabling you to block malicious traffic originating from targeted geographic zones with one click.

#### 3. Human-Machine Challenge Configuration (PoW CC Protection)
* **Description**: Enable CC protection human-machine challenges. When a request triggers the CC protection threshold, the browser renders a silent challenge page, solving a mathematical challenge (hash collision) within several hundred milliseconds. Upon passing, it sets a Cookie and allows subsequent visits. This is seamless to actual users but blocks brute-force scripts and CC tools that do not support JS execution or mathematical computations.
* **Core Parameters**:
    * **Status**: Enable / Disable.
    * **Hash Difficulty**: Controls the computation difficulty (recommending `4` or `5`).
    * **Cookie Expiration**: How long the verification remains valid after passing (e.g., `3600` seconds).
    * **Custom Challenge HTML**: Customize the Loading page style of the challenge to match your business design.

#### 4. Block Response (Block Response)
* **Description**: Define the behavior of the WAF when blocking malicious requests.
* **Configuration**:
    * **Block Status Code**: Customize the HTTP status code returned, e.g., the standard `403` or a fun `418 (I'm a teapot)`.
    * **Block Response Body**: Input custom HTML content shown to blocked attackers (e.g., "WAF Interception: Your request has been logged").

---

### Step 3: Associate the Rule Group with Websites

Once configured, the rule group does not automatically take effect; you need to bind it to specific website configurations.

* **Option A (Recommended)**: In the **"Bind Websites"** Tab of the rule group details, select the websites you wish to apply this rule group to and save.
* **Option B**: Return to **"Website Configuration"**, edit a specific website, and check and bind the rule group in the "Security Protection" section.

> [!NOTE]
> If a rule group is marked as **"Global Rule Group (is_global)"**, it applies to **all websites** hosted on the gateway automatically, requiring no manual binding.

---

### Step 4: Publish & Activate Configurations

1. If you modify **rule group definitions**, **GeoIP scopes**, **PoW CC difficulties**, or **website-to-rule-group bindings**:
   * Click **"Preview Config"** -> **"Publish & Activate"** in the top right corner.
   * Once the Agent pulls and validates the new version, it rewrites local core OpenResty config files (`waf_config.json`, etc.) and gracefully reloads the processes to apply the policies.
2. If you only update **IP group members** (e.g., adding/deleting an IP in a manual IP group, or an automatic IP group aggregates a new set of blocked IPs periodically):
   * **No publication or activation is required!**
   * The Server calculates the new MD5 Checksum of the IP group immediately after updating the database.
   * The control plane **broadcasts the modified IP group members in real-time to all online Agents via WebSocket**. The Agent overwrites the runtime local disk file `waf_ip_groups.json` incrementally.
   * The OpenResty Lua engine calculates the file hash in microseconds when processing new requests. If it detects a Checksum change, it reloads it into the memory dictionary (`ngx.shared`) in real-time. **This entire process requires absolutely no Nginx service reloads, having zero impact on online high-concurrency operations**.
   * Even if the WebSocket connection drops, the Agent reports its local Checksum in every heartbeat cycle, and the Server syncs the differential updates to guarantee synchronization.

---

## WAF Evaluation Flow (Filtering Funnel)

When an external request reaches the OpenResty data plane, the WAF runtime evaluates it in the `access` phase according to the funnel decision chain below. Once a match is made, evaluation terminates:

```text
       Request enters access phase
                  │
                  v
       Get all active rule groups bound to this site (Global + Bound Custom groups)
                  │
                  v
       1. Matches IP whitelist / Whitelist IP group? ──────(Yes)─────► [ Allow (ALLOW) ]
                  │ (No)
                  v
       2. Matches country / province whitelist? ────────(Yes)─────► [ Allow (ALLOW) ]
                  │ (No)
                  v
       3. Matches IP blacklist / Blacklist IP group? ──────(Yes)─────► [ Block (BLOCK) ] ──► Return status & HTML block page
                  │ (No)
                  v
       4. Matches country / province blacklist? ────────(Yes)─────► [ Block (BLOCK) ] ──► Return status & HTML block page
                  │ (No)
                  v
       5. Is PoW CC protection enabled for this site?
                  ├───(Yes)───► [ Validate PoW Cookie ] ──(Passed)──► [ Allow (ALLOW) ]
                  │                   │
                  │              (Not Passed)
                  │                   v
                  │             [ Render PoW Challenge ] ──(Solved)──► Set Cookie & Allow
                  v
       6. No rules triggered, legitimate traffic ─────────────────────► [ Allow (ALLOW) ]
```

---

## Best Practices & Tuning Recommendations

* **Whitelist Precedence & Protection**: Before deploying strict blacklists or regional blocks, we strongly recommend creating a "Trusted IP Group" containing your team's office egress IPs, local development IPs, and third-party callback server IPs (e.g., WeChat or Alipay payment callback addresses), and prioritizing it in the rule group's **whitelist**. This effectively prevents accidental blockages.
* **Reasonably Fine-tune PoW Difficulty**: Human-machine CC challenge hash difficulty (`challenge_difficulty`) is a double-edged sword:
    * Difficulty `3`: Computes almost instantly, providing low protection.
    * Difficulty `4`: Normal phones/low-end browsers solve it in 100-300ms, providing good protection.
    * Difficulty `5`: Requires 500ms-2s, providing strong protection but low-end client browsers might perceive slight loading delays.
    * Difficulty `6` and above: Computes exponentially slower, easily freezing client browser CPUs. **We strongly recommend choosing `4` or `5` in production**.
* **Utilize "Test Rule"**: For automatic IP groups, always click **"Test Rule"** before saving. By inspecting the list of matching IPs in the current window, verify if your Expr expressions thresholds (such as request counts, 404 ratios, etc.) are too broad or too strict, preventing accidental blockages of legitimate users.
* **Isolate Static & Dynamic Blacklists**: Never enter static malicious IPs that require permanent blocks directly into automatic IP groups (since the aggregated list will be overwritten in the next cron cycle). You should add permanent malicious IPs into a dedicated "Manual Blacklist IP Group" and reference both the manual and automatic groups in your rule groups.
