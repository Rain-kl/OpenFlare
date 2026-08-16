# SSO Login Configuration

You will learn: how to configure an OIDC third-party login entry for OpenFlare, fill in the callback URL, and how third-party accounts bind to local users.

OpenFlare connects third-party login through OIDC auth sources. Any service providing standard OIDC Discovery (Google, Keycloak, authentik, Logto, Casdoor, etc.) can be integrated.

After an auth source is configured and enabled, it appears in the third-party account login area on the login page. Users can log in with a third-party account, or bind a third-party account to the current local account while logged in.

## Prerequisites

| Item | Description |
| --- | --- |
| Server access URL | configured in admin「System Settings」->「System Settings」tab ->「General Settings」; must match the address users' browsers actually visit (protocol, domain, port) |
| Auth source name | unique identifier inside OpenFlare, e.g. `company-oidc` |
| Client ID | provided after creating the app on the third-party platform |
| Client Secret | provided after creating the app on the third-party platform |
| OIDC Discovery URL | e.g. `https://idp.example.com/.well-known/openid-configuration` |

The auth source name may only contain letters, digits, hyphens, or underscores, and must start with a letter or digit.

## Callback URL

The Redirect URI / Callback URL on the third-party platform is fixed to:

```text
<server access URL>/login
```

For example, with a server access URL of `https://openflare.example.com`:

```text
https://openflare.example.com/login
```

The callback URL only relates to the「server access URL」and does not include the auth source name. After the third-party platform completes authorization, it redirects here, and the OpenFlare login page uses the authorization code to complete login or binding.

## Configure OIDC Login

1. Create an app or client on the OIDC Provider; choose Web / Confidential Client as the app type.
2. Set the Redirect URI / Callback URL to `<server access URL>/login`.
3. Copy the Client ID and Client Secret.
4. Get the Provider's Discovery URL, usually ending in `/.well-known/openid-configuration`.
5. Log in to the OpenFlare admin panel, go to **「System Settings」**, select the **「Security Settings」** tab, and add an auth source in the **「Auth Source Management」** section.
6. Choose type `OIDC`; fill in the auth source name, display name, Client ID, Client Secret, and OIDC Discovery URL.
7. Scope defaults to `openid profile email`. If the Provider restricts scopes, adjust according to the Provider's allowed values.
8. Save and enable the auth source.

Once enabled, the login page shows the corresponding third-party login button.

## Login and Binding Behavior

When a third-party account returns to OpenFlare, it is handled as follows:

| Scenario | Behavior |
| --- | --- |
| Third-party account already bound to a local user | log in directly |
| User already logged in and initiates third-party authorization | bind to the current local user |
| Third-party account not bound, and registration allowed | auto-create a normal user and bind |
| Third-party account not bound, and registration disabled | require entering an existing local account password to bind |

To allow only existing users to use SSO, turn off user registration. Unbound third-party accounts then enter the bind-existing-account flow.

## Modifying an Auth Source

When editing an auth source, leaving the Client Secret input empty keeps the existing secret; entering a new value overwrites and saves it.

Changing the auth source name does not affect the callback URL, so the third-party platform config doesn't need to change.

## FAQ

### Returns `invalid_scope`

The third-party platform doesn't allow the currently configured scope. The OIDC default scope is `openid profile email`. Adjust the scope on the auth source edit page, or allow the scope on the third-party platform.

### Callback URL mismatch

Check that the Redirect URI / Callback URL on the third-party platform exactly matches `<server access URL>/login`. Protocol, domain, port, and path must all match.

### Login page doesn't show the third-party login button

Check that the auth source is enabled and that the Client ID and Client Secret are saved. OpenFlare validates these fields before enabling the auth source.

### Client Secret saved, but the list doesn't show the plaintext

This is expected. OpenFlare never echoes the Client Secret through the API; it only shows whether a secret is configured.
