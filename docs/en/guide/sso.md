# SSO Login Configuration

You will learn: How to configure GitHub OAuth or standard OIDC login portals for OpenFlare, how to fill in callback URLs, and how third-party accounts bind to existing local users.

OpenFlare supports third-party logins configured via Authentication Sources. Currently, GitHub OAuth and standard OIDC Providers (e.g., Logto, authentik, Keycloak, Casdoor) are supported.

Once an Authentication Source is configured and enabled, it displays in the third-party login section of the login page. Users can log in using their third-party accounts or bind their third-party accounts to their current local account while logged in.

## Prerequisites

Before starting, prepare the following:

| Item | Description |
| --- | --- |
| OpenFlare URL | The actual URL accessed by user browsers, e.g., `https://openflare.example.com` |
| Auth Source Name | Unique internal identifier in OpenFlare, e.g., `github`, `company-oidc` |
| Client ID | Provided after creating an application in the third-party platform |
| Client Secret | Provided after creating an application in the third-party platform |
| OIDC Discovery URL | Required for OIDC only, e.g., `https://idp.example.com/.well-known/openid-configuration` |

**Verify that "System Settings -> General Settings -> Server Address" accurately matches your domain name.**

The Auth Source name can only contain letters, numbers, hyphens, or underscores, and must start with a letter or number. The Auth Source name will appear in the callback URL; if you modify the name after saving, you must simultaneously modify the callback URL on the third-party platform.

## Callback URL

The Redirect URI / Callback URL in third-party platforms is formatted as:

```text
<OpenFlare URL>/oauth/<Auth Source Name>
```

Example:

```text
https://openflare.example.com/oauth/github
https://openflare.example.com/oauth/company-oidc
```

When creating or editing an authentication source in the management console, the form automatically generates the callback URL based on your current browser URL and the Auth Source name you entered.

## Configure GitHub Login

1. Create an OAuth App in GitHub.
2. Fill `Homepage URL` with your OpenFlare URL.
3. Fill `Authorization callback URL` with the callback URL generated in OpenFlare, e.g., `https://openflare.example.com/oauth/github`.
4. Copy the Client ID and Client Secret provided by GitHub.
5. Log into the OpenFlare management console, go to "Settings -> System Settings -> Configure Authentication Sources".
6. Add an authentication source, choosing `GitHub` as the type.
7. Fill in the Auth Source name, display name, Client ID, and Client Secret.
8. The Scope defaults to `user:email`, which usually requires no modification.
9. Save and enable the authentication source.

Once enabled, the corresponding GitHub login button will display on the login page.

## Configure OIDC Login

1. Create an application or client in your OIDC Provider.
2. Select Web / Confidential Client as the application type.
3. Fill `Redirect URI / Callback URL` with the callback URL generated in OpenFlare, e.g., `https://openflare.example.com/oauth/company-oidc`.
4. Copy the Client ID and Client Secret.
5. Retrieve the Provider's Discovery URL, which usually ends with `/.well-known/openid-configuration`.
6. Log into the OpenFlare management console, go to "Settings -> System Settings -> Configure Authentication Sources".
7. Add an authentication source, choosing `OIDC` as the type.
8. Fill in the Auth Source name, display name, Client ID, Client Secret, and OIDC Discovery URL.
9. Scope defaults to `openid profile email`. If the Provider restricts scopes, adjust to values permitted by the Provider.
10. Save and enable the authentication source.

Once enabled, the corresponding OIDC login button will display on the login page.

## Login & Binding Behaviors

Once a third-party account returns to OpenFlare, it is processed according to the following rules:

| Scenario | Behavior |
| --- | --- |
| Third-party account is already bound to a local user | Logs in directly |
| User is already logged in and initiates third-party authorization | Binds to the current local user |
| Third-party account is unbound, and registration is enabled | Automatically creates a standard user and binds |
| Third-party account is unbound, and registration is disabled | Prompts to enter an existing local username and password to complete the binding |

If you want only existing users to use SSO, you can disable user registration. Unbound third-party accounts will then trigger the binding flow.

## Modify Authentication Source

When editing an authentication source, leaving the Client Secret field blank retains the existing secret; entering a new value will overwrite the saved secret.

If you modify the Auth Source name, the callback URL changes accordingly. You must modify the Redirect URI / Callback URL on the third-party platform; otherwise, the third-party platform will deny the callback or return an error.

## Common Problems

### Returns `invalid_scope`

This indicates that the third-party platform does not permit the configured Scope. OIDC defaults to `openid profile email`, and GitHub defaults to `user:email`. Adjust the Scope in the authentication source edit page or configure the third-party platform to permit the scope.

### Callback Address Mismatch

Verify if the Redirect URI / Callback URL configured in the third-party platform matches the prompt in the OpenFlare form exactly. The protocol, domain, port, and path must match.

### Third-party Login Button Not Showing on Login Page

Verify if the authentication source is enabled and confirm that the Client ID and Client Secret are saved. OpenFlare validates these fields before enabling the source.

### Client Secret Saved but Not Displayed in Clear Text

This is expected behavior. OpenFlare does not echo the Client Secret back via API, displaying only whether the secret is configured.
