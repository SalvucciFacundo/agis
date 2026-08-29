# Webhook Spec

## Purpose

Provide an HTTP webhook listener server for ingesting external events securely with constant-time HMAC-SHA256 signature verification, body size limits, and dispatching to Brain.Step with optional Gateway notification target delivery.

## Requirements

### Requirement AGIS-M6-WBH-001: Webhook HTTP Listener Server
The system MUST provide a Webhook Server (`internal/webhook/`) that listens on a configured host and port (e.g. `127.0.0.1:8080`) and handles HTTP POST requests at the configured endpoint path (default `/webhook` or `/events`).
- The server MUST respond with `200 OK` on valid accepted payloads.
- The server MUST respond with `405 Method Not Allowed` on non-POST requests.
- The server MUST support graceful shutdown via `context.Context` without terminating active connections abruptly.
- The server MUST enforce a 1MB maximum body limit.

#### Scenario: HTTP POST request received on webhook path
- GIVEN a running Webhook HTTP server on port 8080 with path `"/webhook"`
- WHEN a valid HTTP POST request is sent to `http://127.0.0.1:8080/webhook`
- THEN the server processes the event and returns HTTP status `200 OK`

#### Scenario: HTTP GET request rejected
- GIVEN a running Webhook HTTP server
- WHEN an HTTP GET request is made to `"/webhook"`
- THEN the server returns HTTP status `405 Method Not Allowed`

### Requirement AGIS-M6-WBH-002: HMAC-SHA256 Signature Verification
When a webhook secret is configured in `config.yaml`, the server MUST verify the payload integrity and authenticity using HMAC-SHA256:
- The signature MUST be extracted from the `X-Hub-Signature-256` or `X-Signature` header (supporting `sha256=` prefix).
- The server MUST compute the HMAC-SHA256 of the raw request body using the configured secret and compare it to the header signature using constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
- Requests with missing, invalid, or mismatched signatures MUST be rejected immediately with `401 Unauthorized` before reading or executing payload content.

#### Scenario: Valid HMAC-SHA256 signature accepted
- GIVEN a webhook configured with secret `"secret-token-123"`
- WHEN an HTTP POST arrives with body `{"event":"alert"}` and valid header `X-Hub-Signature-256: sha256=<valid_hmac>`
- THEN the server validates the signature and accepts the request

#### Scenario: Invalid signature rejected with 401
- GIVEN a webhook configured with secret `"secret-token-123"`
- WHEN an HTTP POST arrives with an invalid or tampered signature header
- THEN the server rejects the request with HTTP status `401 Unauthorized` and does not process the body

### Requirement AGIS-M6-WBH-003: Webhook Event Ingestion and Dispatch
Upon signature validation, the Webhook Server MUST parse the JSON event payload and dispatch it:
- The event MUST be dispatched to `core.Brain.Step` with a constructed prompt (e.g. `"Webhook event received: <payload>"`).
- The execution MUST use the configured `default_session_id` or an ephemeral session `webhook:<event_type>`.
- If configured with a gateway delivery target, the brain's response MUST be forwarded to the Gateway Multiplexer for outbound delivery.

#### Scenario: Webhook event triggers Brain turn
- GIVEN an accepted webhook event payload `{"alert": "high_cpu", "server": "app-01"}`
- WHEN the event is dispatched
- THEN `Brain.Step` executes the event prompt and logs or sends the response to the target gateway recipient
