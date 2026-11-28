# SSO Login Configuration

Integrate third-party identity providers such as Google, Keycloak, authentik, Logto, or Casdoor via standard OIDC Discovery.

## 1. Prerequisites

| Item | Requirement |
| :--- | :--- |
| Server access URL | Configured in **Admin** > **System settings** > **System** > **General**; must match the exact scheme, host, and port visited by users |
| Auth source name | Unique identifier (such as `company-oidc`) starting with an alphanumeric character using letters, digits, `-`, or `_` |
| Client ID / Secret | Application credentials issued by your OIDC provider |
| OIDC Discovery URL | Metadata endpoint typically ending in `/.well-known/openid-configuration` |

All auth sources share one callback URL (Redirect URI) without appending the source name:

```text
<server access URL>/login
```

## 2. Configure an OIDC Auth Source

1. Create a Web / Confidential client in your provider with Redirect URI `<server access URL>/login`.
2. Open **Admin** > **System settings** > **Security** > **Auth Source Management**, click **Add auth source**, and select `OIDC`.
3. Fill in Name, Display Name, Client ID, Client Secret, Discovery URL, and Scope (default `openid profile email`), then save and enable (leave Secret blank when editing to preserve).

## 3. Account Binding & Troubleshooting

| Scenario | Behavior |
| :--- | :--- |
| External identity bound | Signs straight into the linked local account |
| Signed-in user authorizes | Binds external identity to the active local account |
| Unbound & registration open | Auto-provisions a standard user and binds it |
| Unbound & registration closed | Prompts for local credentials to bind once |

Identities bind by `auth_source + sub`, preserving links across upgrades.

| Symptom | Fix |
| :--- | :--- |
| `invalid_scope` | Align Scope on the auth source or permit `openid profile email` in your provider |
| Redirect URI mismatch | Verify exact scheme, host, port, and `/login` path against `<server access URL>/login` |
| Error on callback redirect | Check `POST /api/v1/oauth/callback` in DevTools and restart login from `/login` rather than refreshing |
| Provider button missing | Confirm **Allow OIDC login** is enabled under **Security** and the source is active with saved credentials |
| Client Secret hidden | Expected behavior; plaintext secrets are never returned after saving |
