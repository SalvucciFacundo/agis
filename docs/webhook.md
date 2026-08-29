# Webhook Event Ingestion Guide

AGIS provides a secure, built-in **HTTP Webhook Listener Server** (`internal/webhook`) for ingesting real-time alerts, notifications, and events from third-party services (such as GitHub, Stripe, Datadog, Sentry, or custom applications).

Incoming HTTP POST events are authenticated via constant-time HMAC-SHA256 signature verification, processed through `core.Brain.Step`, and can optionally forward AI-generated responses to external chat channels via the Gateway Multiplexer.

---

## Architecture & Security Workflow

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        WEBHOOK LISTENER SERVER                         │
│                                                                        │
│   External Service (GitHub, Sentry, Stripe, Monitoring)                │
│                           │                                            │
│                           ▼ (HTTP POST /webhook with HMAC Header)      │
│   ┌────────────────────────────────────────────────────────────────┐   │
│   │ 1. Method Check (POST only, 405 on others)                     │   │
│   │ 2. Body Size Limit (1MB maximum payload enforcement)           │   │
│   │ 3. HMAC-SHA256 Verification (crypto/subtle.ConstantTimeCompare)│   │
│   │    -> 401 Unauthorized immediately if signature invalid        │   │
│   └───────────────────────────────┬────────────────────────────────┘   │
│                                   │                                    │
│                                   ▼ Valid Event Payload                │
│   ┌────────────────────────────────────────────────────────────────┐   │
│   │ 4. Prompt Synthesis ("Webhook event received: <payload>")      │   │
│   │ 5. Session Routing (default_session_id or webhook:<event>)     │   │
│   │ 6. Brain Execution (Brain.Step with AutoDenyApprover)          │   │
│   └───────────────────────────────┬────────────────────────────────┘   │
│                                   │                                    │
│                                   ▼ Optional AI Notification           │
│   ┌────────────────────────────────────────────────────────────────┐   │
│   │ 7. Gateway Multiplexer Delivery (Telegram / Discord target)    │   │
│   └────────────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────────────┘
```

---

## Configuration Schema

Webhooks are configured under the `webhook` block in `config.yaml`:

```yaml
webhook:
  enabled: true
  host: "127.0.0.1"                     # Listening interface (use 0.0.0.0 for public)
  port: 8080                            # Listening port
  path: "/webhook"                      # Endpoint path for incoming HTTP POST
  secret: "your-hmac-sha256-secret-key" # Secret key for HMAC signature verification
  default_session_id: "webhook-events"  # Target conversation session ID
  target:                               # Optional: deliver AI response to chat
    adapter: "telegram"
    recipient: "123456789"
```

---

## HMAC-SHA256 Signature Verification

To prevent unauthorized event injection and replay attacks, AGIS enforces constant-time HMAC-SHA256 signature verification:

1. The sending service must calculate the HMAC-SHA256 of the raw request payload using the shared secret.
2. The signature must be passed in one of the following HTTP headers:
   - `X-Hub-Signature-256: sha256=<hex_hmac>` (GitHub standard)
   - `X-Signature: <hex_hmac>`
3. AGIS calculates the expected HMAC and validates it using `crypto/subtle.ConstantTimeCompare` to eliminate timing side-channel vulnerabilities.
4. If the signature is missing, invalid, or tampered with, the server rejects the request with **`401 Unauthorized`** without parsing the payload or invoking the Brain.

---

## Starting the Webhook Server

```bash
# Start webhook server with settings from config.yaml
./bin/agis webhook run

# Start webhook server overriding host and port via CLI flags
./bin/agis webhook run --host 0.0.0.0 --port 9090 --path /api/v1/events
```

---

## Sending a Test Webhook Request

### Example: Using `curl` with OpenSSL HMAC Calculation

```bash
# 1. Payload & Secret
SECRET="your-hmac-sha256-secret-key"
PAYLOAD='{"event": "deployment_failed", "service": "payment-api", "cluster": "us-east-1"}'

# 2. Compute HMAC-SHA256
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')

# 3. Send HTTP POST
curl -X POST http://127.0.0.1:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=${SIGNATURE}" \
  -d "$PAYLOAD"
```

Response:
```text
HTTP/1.1 200 OK
Content-Type: application/json

{"status": "accepted"}
```

The server immediately dispatches the event to `core.Brain.Step`, analyzes the alert, and routes any generated summary to the configured notification target (e.g. your Telegram or Discord alert channel).
