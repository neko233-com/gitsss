# Reporting security issues

Gitsss takes security issues seriously. Do not disclose a vulnerability in a
public issue, commit, log, screenshot, or chat message.

## Private reporting

For the public source repository, use the private security-advisory workflow:

<https://github.com/neko233-com/gitsss/security/advisories/new>

For an internal deployment, report to the service owner through the approved
private channel and include the affected instance version, configuration
surface, reproduction steps, impact, and a safe mitigation. Do not include
passwords, tokens, invitation URLs, or repository contents in the report.

If the issue affects a migration or compatibility path, state whether the
source data, Git protocol, LFS objects, API contract, or storage paths are
affected. Preserve evidence in an access-controlled location.

## Response expectations

Maintainers will acknowledge receipt, assess severity and exposure, prepare a
fix or mitigation, and coordinate disclosure timing with the reporter. Rotate
any credential that may have been exposed before investigating further.
