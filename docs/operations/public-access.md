# Public access and login protection

GitSSS can keep its existing LAN Git URLs while a TLS reverse proxy publishes a
separate HTTPS origin. Public access does not enable registration or grant access
to private repositories. Keep self-registration disabled for enterprise sites.

## Adaptive authentication

`[security] LOGIN_PROTECTION_ENABLED = true` is the default. Protection runs
before expensive HTTP password verification, with atomic reservations for
concurrent requests. It applies to the browser login, account linking, HTTP Basic
Git/API/LFS passwords, and password checks made by the account settings pages.

- Browser login requests a local, single-use image CAPTCHA after 5 failures for a
  login identifier or account/source pair in 15 minutes. It also challenges after
  10 identifier attempts, 5 pair attempts, or 60 source attempts in one minute,
  and after 30 failed attempts from a source in 15 minutes. A fresh page can still
  show the normal form; a POST must pass the current server-side policy.
- Correct credentials recently verified by this process bypass these attempt
  counters for 15 minutes. Every request still verifies the password, account
  status, and configured second factor; this is not an authentication cache.
  Only a process-keyed HMAC is retained, never plaintext passwords. Password
  revocation still takes effect on the next request.
- New HTTP Basic password attempts have one-minute budgets of 10 per
  identifier/source pair, 30 per identifier, and 120 per source. Excess requests
  receive HTTP `429` with `Retry-After`. Browser CAPTCHA remains a recovery path;
  a successful browser password check also makes that credential eligible for
  the normal Git HTTP path. Personal, OAuth and Actions tokens are checked first
  and do not consume password budgets. SSH authentication is unchanged.
- A newly verified credential clears only its identifier and pair counters. It
  does not reset the shared source history, so a successful attacker-owned login
  cannot clear password-spraying detection for everyone else.
- TOTP and recovery code entry share 10 attempts per account per minute, after a
  valid password establishes the account. TOTP replay protection is unchanged.
- There are at most 16 in-flight password verifications, 32,768 tracking buckets,
  and 4,096 credential HMACs. Saturation returns a short retry response or requires
  CAPTCHA; it does not allocate unbounded per-attacker state. Counters expire in
  memory and reset on process restart. Multiple server processes have independent
  budgets; this implementation does not provide a shared cluster-wide limit.

There is no permanent IP ban or account lockout. Colleagues sharing NAT retain
existing sessions and tokens; a new browser login may need CAPTCHA during a
source-wide attack. A first-time Basic password request can briefly require a
retry during an attack on the same identifier. Prefer tokens or SSH keys for
unattended Git clients. CAPTCHA and rate limiting reduce automated guessing;
strong unique passwords and optional TOTP remain important. They do not provide
network-level DDoS protection.

Disabling `LOGIN_PROTECTION_ENABLED` restores the previous unrestricted HTTP
password path. It does not change accounts, password hashes, SSH keys, Git data,
LFS objects, database schema, storage layout or external provider configuration.

## Proxy trust boundary

Only list the exact IPs/networks of trusted proxy hops. Do not use `*` for a
public deployment. GitSSS walks `X-Forwarded-For` from right to left, stops at the
first untrusted hop, and never processes more than `REVERSE_PROXY_LIMIT` hops.
Comma spacing does not affect parsing. A valid `X-Forwarded-For` chain takes
precedence over `X-Real-IP`. Invalid/non-IP entries do not become source keys.
Direct clients cannot change their address or public URL scheme with forwarding
headers. Invalid proxy configuration can collapse many visitors onto one source;
verify an actual external request before enabling source-based controls.

At the public TLS terminator, **replace** incoming forwarding headers rather than
appending client-controlled values:

```nginx
proxy_set_header Host $host;
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $remote_addr;
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Original-Proto $scheme;
proxy_cookie_flags ~ secure;
```

An frp HTTP tunnel may replace `X-Forwarded-Proto` with its internal HTTP scheme.
In that case use a separately overwritten header through the trusted tunnel:

```ini
[security]
LOGIN_PROTECTION_ENABLED = true
REVERSE_PROXY_LIMIT = 2
REVERSE_PROXY_TRUSTED_PROXIES = 127.0.0.1/32,::1/128
REVERSE_PROXY_PROTO_HEADER = X-Original-Proto
```

The protocol header defaults to `X-Forwarded-Proto`; only `http` or `https` is
accepted. The example hop count assumes a local TLS proxy plus a local frp hop;
match the observed chain. A TLS listener behind an Nginx stream Unix socket needs
`set_real_ip_from unix:` and `real_ip_header proxy_protocol` only when that socket
actually receives trusted PROXY protocol. Do not trust PROXY data from arbitrary
public clients.

Keep the frps HTTP backend port private to the TLS proxy, including IPv6. Otherwise
clients can bypass TLS and inject their own trusted headers through the tunnel.
Restrict only the backend port, preserving SSH, frp control and existing TCP
forwarding ports. Keep the restriction persistent and document its rollback.
The HTTPS virtual host should set `Secure` on cookies and can send HSTS for that
host. Keep LAN HTTP cookies independent when retaining old LAN origins.

## Acceptance and rollback

Use isolated synthetic accounts to test incorrect attempts, simultaneous requests,
CAPTCHA consumption/replay, same-NAT valid logins, token access during a password
limit, and MFA limits. Never brute-force a production user's account for testing.
Verify external forwarding-header spoof attempts, HTTPS cookies and links, and
that the backend port is not publicly accessible. Recheck existing HTTP/SSH refs
and LFS authentication/downloads and other service processes after deployment.

This changes the trust parsing of proxy headers and adds HTTP `429` responses for
excess password requests. Existing JSON error shapes remain unchanged. Operators
with unusual proxy chains must update their explicit hop/trust settings. Rollback
is the previous binary and configuration; no database or Git/LFS migration is
required. Restore the previous virtual-host configuration only when its original
security properties are acceptable; the backend firewall rule can be removed
independently of the application binary.
