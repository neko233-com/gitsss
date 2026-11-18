# Agent work analysis

Automatic daily delivery and historical resend have been removed. Organization settings still hold the Responses API base URL, model, API key, `omit_store` option and Git author aliases. Credentials remain server-side. Saving settings clears the obsolete Feishu webhook and disables the stored schedule.

Organization owners can generate an on-page preview by selecting members and/or role labels. Preview results are never sent and do not create delivery receipts. Site administrators can access these settings too.

## Self-service API

Authenticated users can request an analysis of their own commits:

```http
POST /api/v1/user/daily-report/analyze
Authorization: token <USER_API_TOKEN>
Content-Type: application/json

{"organization_id": 123}
```

The API token needs user scope. The organization ID must identify an organization the caller belongs to and has configured with an agent. The server analyzes only the caller's commits, and only in organization repositories where the caller can read code. Callers cannot select another account. The response contains `organization_id`, `account`, `full_name` and `summary`; if there are no reportable changes, the summary says there are no confirmed personal business outcomes. Concurrent requests for the same user and organization return HTTP 409. The analysis is not persisted or sent to Feishu.

The request uses the latest complete reporting window: 18:30 on the previous day through 18:30 on the current day, Asia/Shanghai. The configured provider receives commit IDs, repository IDs, commit messages, changed paths and bounded source diff excerpts. These may contain confidential code or credentials and are not automatically redacted; approve the provider for this data before enabling self-service analysis.

## Attribution and limits

Commits with more than one Git parent are excluded before analysis. Clearly marked merge, sync, backfill and cherry-pick transport records are also ignored. Author email and organization-configured aliases determine ownership. Other commits are compared with their direct parent; root commits use the empty tree. Commit content is data, never instructions. If a repository or commit cannot be read, analysis fails instead of presenting the commit message as verified work.

Each commit contributes at most 16 KiB of diff and each person's total diff budget is 256 KiB, divided across their commits. A maximum of 256 commits per analysis is accepted. Binary files are represented by change markers. Large, generated, binary or non-text changes may not be fully represented, so generated summaries still require human review.

Existing organization schedule and webhook values are ignored by the new version; saving the settings removes them. Rolling back to an older binary may restore automatic delivery from old configuration, so disable the old schedule before rollback if delivery must remain stopped. No Git, LFS, database schema or repository migration is required.
