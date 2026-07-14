# Notifications v1

gitsss includes email, Feishu and WeCom adapters backed by an encrypted transactional outbox. Channel configuration and message subject/body are sealed with the instance master key; list APIs expose only masked destinations.

Each channel has an independently persisted event scope. Operators can select pipeline failure/success and takeover failure/success without re-entering the channel secret. Omitted scopes from older configuration documents are upgraded in memory to all four events; an explicitly empty scope keeps the channel connected for manual tests without automatic event delivery. Pipeline completion and takeover cutover/failure enqueue only to channels that opted into the matching event.

## Delivery guarantees

- Enqueue is idempotent per channel and caller key.
- A persistent claim is written before network I/O. Delivery authorization must reach the audit log before the adapter runs.
- The server starts a background dispatcher immediately and polls every 15 seconds; administrators do not need to call a dispatch endpoint.
- Failures retry after 1, 2 and 4 minutes, then enter the dead-letter state on the fourth failed attempt.
- After fixing a channel, an administrator can explicitly reset a dead letter to pending. The retry resets its attempt budget, persists before becoming visible, and is audit logged; pending, delivering, retrying and delivered messages cannot be reset accidentally.
- A process crash recovers an unknown delivery with explicit at-least-once semantics. A stuck claim expires after 15 minutes.
- Pipeline completion automatically enqueues one idempotent message for every configured channel. The background worker performs delivery.
- Configuration is not made visible until persistence and audit both succeed. An audit failure restores the previous encrypted document or deletes a new document.
- Event-scope changes and dead-letter retries use the same persist-then-audit transaction with rollback on audit failure.

Webhook configuration accepts only the official HTTPS Feishu and WeCom endpoint shapes. Feishu signatures use HMAC-SHA256; response bodies and platform error codes are checked. Webhook diagnostics never return URLs or secrets.

Email supports implicit TLS on port 465 and requires STARTTLS on other ports with TLS 1.2 or newer and hostname verification. Plain SMTP is rejected. Envelope addresses are parsed separately from display names, subjects are encoded, header injection is rejected, and transport errors are reduced to bounded non-secret summaries.
