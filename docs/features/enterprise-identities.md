# Enterprise identity binding

GitSSS supports password sign-in and configured Google, Feishu and WeChat website identities. Providers appear only after an administrator configures and activates an authentication source. Existing account identifiers, passwords and the legacy Google provider key `gplus` remain unchanged. Public registration and OAuth automatic registration should remain disabled for an internal enterprise deployment.

## Administrator setup

Use **Administration → Authentication Sources → Add source**, type OAuth2. Choose Google, 飞书 (`feishu`) or 微信 (`wechat`). Enter the application's client ID and secret in this administrator-only form. Never commit these values or include them in reports. Configure the exact callback URI shown by GitSSS, including the authentication-source name, under the provider's application security settings. A Feishu notification bot webhook is not a login application credential.

For example, a source named `Feishu` uses `https://git.example.com/user/oauth2/Feishu/callback`. Use a stable, provider-approved HTTPS hostname and a trusted certificate; Google and WeChat application restrictions cannot be bypassed by configuring a raw private IP in GitSSS. Browser access to the intranet callback and server access to the provider APIs are required. Provider application approval, client credentials, scopes and redirect allowlists must be available before real authorization can be accepted. Configuration alone is not evidence of a successful external login.

Google requests email identity; Feishu uses its authorization/user-info flow, and WeChat uses the Open Platform website QR authorization with `snsapi_login`. Request only the user-info permissions needed by the selected application. Google is not renamed internally from `gplus`, so existing bindings survive upgrades.

## User binding and replacement

Open **Settings → Security → Linked accounts** while signed in. Bind providers independently; Google, Feishu and WeChat can coexist on one GitSSS account. A verified identity cannot be taken from another account. Replacement is an explicit action that starts a new provider authorization. The binding intent is scoped to the current user/source/session, expires after ten minutes and is consumed on callback. The old provider identity remains valid until the new identity has been authenticated and the database transaction succeeds. Other provider bindings remain unchanged. Existing OAuth-primary identity mappings are updated consistently during replacement/removal.

Removing a native identity checks that a working password or another active authentication method remains. Provider names and returned account names/emails are displayed as text, never trusted HTML. Old OpenID and passwordless passkey entry points are removed from the standard sign-in/security UI. `ENABLE_PASSKEY_AUTHENTICATION` now defaults to false; deployment explicitly disables this and OpenID sign-in/registration. Existing OpenID/WebAuthn records are retained, and TOTP remains available. No keys are silently deleted.

## Google email aliases

Only a server-fetched Google identity with boolean `email_verified` or `verified_email` true can add an activated email alias. Other providers and string-valued claims cannot mark email verified. The email must satisfy the site's email rules and globally unique, case-insensitive ownership. Existing aliases belonging to the same user may be activated; another user's address is never reassigned. The user's chosen primary address stays unchanged. Previous Google aliases remain for commit attribution until removed through Account settings. The current Google identity's email is also visible under Linked accounts. Established provider sign-in remains available if an alias cannot be added, with an account-settings warning; a failed new binding rolls its email and identity changes back together.

## Compatibility and acceptance

No schema migration, provider-key rename, repository/LFS path, SSH identity or object-byte change is required. New definitions use the existing OAuth source and external-login tables. Bind/unlink transactions serialize per account without rewriting profile fields or timestamps. Roll back the binary and restore the documented login configuration if required; retain existing authentication sources and account data.

Local acceptance covers authorization URL/state construction without leaking secrets, verified-email claim handling, alias uniqueness/primary preservation, identity ownership, atomic replacement and last-method protection. Real Google/Feishu/WeChat browser consent requires configured applications and is a separate acceptance layer. Do not enable a dummy source or claim a successful external login from mocked provider responses.

References: [Google OAuth web applications](https://developers.google.com/identity/protocols/oauth2/web-server), [Google verified email](https://developers.google.com/identity/openid-connect/reference), [Feishu authentication](https://open.feishu.cn/document/server-docs/authentication-management), [WeChat Open Platform](https://open.weixin.qq.com/).
