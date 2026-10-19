# Enterprise identity binding

GitSSS supports password sign-in and configured Google, Feishu, WeChat and Discord website identities. Sign-in buttons appear only after an administrator activates a source with the required client ID and secret. Unconfigured or inactive channels stay hidden. Security settings always explain email/password and linked-channel setup. Existing account identifiers, passwords and the legacy Google provider key `gplus` remain unchanged. Public registration and OAuth automatic registration should remain disabled for an internal enterprise deployment.

## Administrator setup

Use **Administration → Authentication Sources → Add source**, type OAuth2. Choose Google (`gplus`), 飞书 (`feishu`), 微信 (`wechat`) or Discord (`discord`). Enter the application's client ID and secret in this administrator-only form. Never commit these values or include them in reports. Configure the exact callback URI shown by GitSSS, including the authentication-source name, under the provider's application security settings. A Feishu notification bot webhook is not a login application credential.

For example, a source named `Feishu` uses `https://git.example.com/user/oauth2/Feishu/callback`. Use a stable, provider-approved HTTPS hostname and a trusted certificate; Google and WeChat application restrictions cannot be bypassed by configuring a raw private IP in GitSSS. Browser access to the intranet callback and server access to the provider APIs are required. Provider application approval, client credentials, scopes and redirect allowlists must be available before real authorization can be accepted. Configuration alone is not evidence of a successful external login.

Google always requests account selection (`prompt=select_account`), including when the browser already has a Google session; the login route starts a fresh authorization instead of silently reusing a cached provider identity. WeChat redirects to the Open Platform QR login page (`/connect/qrconnect`). Discord supports the same protected bind, replace and unlink operations as the other native channels. Google requests email identity; Feishu uses its authorization/user-info flow, and WeChat uses the Open Platform website QR authorization with `snsapi_login`. Request only the user-info permissions needed by the selected application. Google is not renamed internally from `gplus`, so existing bindings survive upgrades.

## User binding and replacement

Open **Settings → Security → Linked accounts** while signed in. Bind providers independently; Google, Feishu, WeChat and Discord can coexist on one GitSSS account. A verified identity cannot be taken from another account. Replacement is an explicit action that starts a new provider authorization. The binding intent is scoped to the current user/source/session, expires after ten minutes and is consumed on callback. The old provider identity remains valid until the new identity has been authenticated and the database transaction succeeds. Other provider bindings remain unchanged. Existing OAuth-primary identity mappings are updated consistently during replacement/removal.

Removing a native identity checks that a working password or another active authentication method remains. Provider names and returned account names/emails are displayed as text, never trusted HTML. Old OpenID and passwordless passkey entry points are removed from the standard sign-in/security UI. `ENABLE_PASSKEY_AUTHENTICATION` now defaults to false; deployment explicitly disables this and OpenID sign-in/registration. Existing OpenID/WebAuthn records are retained, and TOTP remains available. No keys are silently deleted.

## Google email aliases

Only a server-fetched Google identity with boolean `email_verified` or `verified_email` true can add an activated email alias. Other providers and string-valued claims cannot mark email verified. The email must satisfy the site's email rules and globally unique, case-insensitive ownership. Existing aliases belonging to the same user may be activated; another user's address is never reassigned. The user's chosen primary address stays unchanged. Previous Google aliases remain for commit attribution until removed through Account settings. The current Google identity's email is also visible under Linked accounts. Established provider sign-in remains available if an alias cannot be added, with an account-settings warning; a failed new binding rolls its email and identity changes back together.

## Compatibility and acceptance

No schema migration, provider-key rename, repository/LFS path, SSH identity or object-byte change is required. New definitions use the existing OAuth source and external-login tables. Bind/unlink transactions serialize per account without rewriting profile fields or timestamps. Roll back the binary and restore the documented login configuration if required; retain existing authentication sources and account data.

Local acceptance covers authorization URL/state construction without leaking secrets, verified-email claim handling, alias uniqueness/primary preservation, identity ownership, atomic replacement and last-method protection. Real Google/Feishu/WeChat browser consent requires configured applications and is a separate acceptance layer. Do not enable a dummy source or claim a successful external login from mocked provider responses.

References: [Google OAuth web applications](https://developers.google.com/identity/protocols/oauth2/web-server), [Google verified email](https://developers.google.com/identity/openid-connect/reference), [Feishu authentication](https://open.feishu.cn/document/server-docs/authentication-management), [WeChat Open Platform](https://open.weixin.qq.com/).

## Intranet callback sites

Each OAuth source has an optional **Callback site URL**. Its precedence is source
`CallbackURLBase`, then `[oauth2_client] CALLBACK_URL_BASE`, then existing
`[server] ROOT_URL`. New values accept HTTPS or HTTP on a private IP, loopback IP or
localhost, without credentials, query or fragment. A subpath is retained. Empty
settings preserve existing callbacks. Administrator forms show the effective base
and source name used by the backend. This changes only OAuth callbacks; Git clone
URLs, SSH, LFS, existing remotes and the canonical ROOT_URL remain unchanged.

For an internal Feishu source named `feishu`, the callback can be
`http://192.168.1.10:5030/user/oauth2/feishu/callback`, if that URL is accepted in the
Feishu application's redirect allowlist and reachable from the user's browser.
Use the same site origin for starting sign-in/binding and returning from the
provider: browser sessions are not transferred across hostnames. Google website
applications require an approved HTTPS hostname (with the provider's localhost
exception for development), so configure their own HTTPS callback site and visit
that site to sign in/bind. Installing the Feishu desktop client does not supply an
application client ID/secret or application approval.

A Feishu enterprise app needs its own App ID/Secret, web login capability,
redirect allowlist, basic user identity permissions, published version and an
approved user availability range. No chat, contact-directory, drive or message
permission is required by this login flow. Enter secrets directly in the protected
administration form; secret inputs are masked. Keep real tenant configuration out
of the public source repository. Bot webhooks remain independent.

Unconfigured provider links are not created as placeholders. Real provider consent
and enterprise approval remain separate from local authorization/state/binding
verification. Adding the callback field is an additive JSON configuration change;
older binaries ignore it and revert to ROOT_URL for callbacks, so disable affected
sources or restore compatible callback configuration when rolling back.

## Provider application manuals

Native channels prioritize verified linking to an existing account, including when
generic OAuth automatic registration is enabled. Disabled/internal-only registration
hides the signup tab and form. Expected identity conflicts show a binding error;
an expired linking session returns to sign-in instead of a server error. Feishu's
basic user-info response can omit employee `user_id`; its server-returned `open_id`
is then used, while existing nonempty employee identifiers remain compatible.

See [channel setup guides](../第三方渠道接入/README.md) and the [HTML manual](../site/identity-channels.html) for application credentials, exact redirect setup, enterprise publication and availability, protected binding, real login acceptance and provider-specific failure cases. Real tenant identifiers and secrets belong only in deployment configuration.
