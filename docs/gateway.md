# Chat Gateway Guide (Telegram, Discord, Slack & WhatsApp)

AGIS features a built-in **Gateway Multiplexer** (`internal/gateway`) that allows your autonomous agent to connect simultaneously to external chat platforms, including **Telegram**, **Discord**, **Slack**, and **WhatsApp**, while sharing the exact same conversation memory, skills, identity, and tool execution engine as the local TUI.

---

## Architecture & Security Posture

```text
┌────────────────────────────────────────────────────────────────────────┐
│                          GATEWAY MULTIPLEXER                           │
│                                                                        │
│   ┌───────────────────────────┐     ┌───────────────────────────────┐  │
│   │     Telegram Adapter      │     │        Discord Adapter        │  │
│   │   (@BotFather API Poll)   │     │      (Discord Gateway WS)     │  │
│   └─────────────┬─────────────┘     └───────────────┬───────────────┘  │
│                 │                                   │                  │
│                 ▼                                   ▼                  │
│         [ Static User Allowlist: Fail-Closed Drop on Mismatch ]        │
│                                     │                                  │
│                                     ▼                                  │
│             [ Session Routing (gateway:<platform>:<id>) ]              │
│                                     │                                  │
│                                     ▼                                  │
│       [ core.Brain.Step with Non-Interactive AutoDenyApprover ]        │
└─────────────────────────────────────┬──────────────────────────────────┘
                                      │
                                      ▼
                        Outbound Chunked Reply Delivery
```

### Security Guardrails:
1. **Static User Allowlist**: Incoming messages from user IDs not listed in your `config.yaml` are dropped immediately before invoking the brain or allocating session memory.
2. **Non-Interactive AutoDeny Approver**: Because chat platforms operate without an interactive TUI terminal prompt, any tool call requiring interactive approval (`DecisionAsk`) is automatically denied under the `sandbox` tier. Commands with persistent `always` allow rules execute normally.
3. **Session Key Isolation**: Conversations are partitioned by platform and chat ID (`gateway:telegram:<chatID>` or `gateway:discord:<channelID>`), preserving distinct conversation histories across chats.

---

## 1. Telegram Adapter Setup

### Step 1: Create a Bot
1. Open Telegram and search for [@BotFather](https://t.me/BotFather).
2. Send `/newbot` and follow instructions to name your bot and choose a username.
3. Copy the generated **HTTP API Token** (e.g. `123456789:ABCdefGhIJKlmNoPQRstuVWxYz`).

### Step 2: Find your Telegram User ID
1. Search for [@userinfobot](https://t.me/userinfobot) on Telegram and start it.
2. Note your numeric User ID (e.g. `987654321`).

### Step 3: Configure `config.yaml`
```yaml
gateway:
  enabled: true
  telegram:
    enabled: true
    token: "123456789:ABCdefGhIJKlmNoPQRstuVWxYz"
    allowlist:
      - "987654321"   # Your numeric Telegram user ID
```

### Message Chunking:
Telegram enforces a 4096-character limit per message. AGIS automatically splits longer responses along UTF-8 rune boundaries without dropping text or breaking emojis.

---

## 2. Discord Adapter Setup

### Step 1: Create a Discord Application & Bot
1. Navigate to the [Discord Developer Portal](https://discord.com/developers/applications).
2. Click **New Application**, give it a name (e.g. `AGIS Agent`), and go to the **Bot** tab.
3. Click **Reset Token** and copy the bot token.
4. Under **Privileged Gateway Intents**, enable:
   - **Message Content Intent**
   - **Server Members Intent** (optional)
5. Under **OAuth2 > URL Generator**, select scopes `bot` and permissions `Send Messages`, `Read Messages/View Channels`, `Read Message History`. Copy the generated URL to invite the bot to your server.

### Step 2: Find your Discord User ID
1. In Discord, go to **Settings > Advanced > Developer Mode** and turn it ON.
2. Right-click your username and select **Copy User ID** (e.g. `112233445566778899`).

### Step 3: Configure `config.yaml`
```yaml
gateway:
  enabled: true
  discord:
    enabled: true
    token: "your-discord-bot-token"
    allowlist:
      - "112233445566778899"  # Your Discord user ID
```

### Message Chunking:
Discord enforces a 2000-character limit per message. AGIS automatically chunks messages into parts under 2000 runes and sends them in sequential order.

---

## 3. Slack Adapter Setup

### Step 1: Create a Slack App
1. Go to [Slack API: Your Apps](https://api.slack.com/apps) and click **Create New App** (from scratch).
2. Under **Basic Information > App Credentials**, copy the **Signing Secret**.
3. Under **OAuth & Permissions**, add bot token scopes (`chat:write`, `app_mentions:read`, `channels:history`, `im:history`).
4. Install the App to your workspace and copy the **Bot User OAuth Token** (`xoxb-...`).
5. Under **Event Subscriptions**, enable events and set the Request URL to `https://<your-domain>/slack/events` (or test with local tunneling). Subscribe to bot events `message.channels`, `message.im`, `app_mention`.

### Step 2: Configure `config.yaml`
```yaml
gateway:
  enabled: true
  slack:
    enabled: true
    bot_token: "xoxb-your-bot-token"
    signing_secret: "your-signing-secret"
    listen_addr: ":3002"
    allowed_users:
      - "U12345678"  # Your Slack User ID
```

### Security & Chunking:
- **HMAC Verification**: All inbound webhooks verify `X-Slack-Signature` using constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
- **Replay Mitigation**: Rejects requests older than 300 seconds based on `X-Slack-Request-Timestamp`.
- **Chunking**: Messages over 4000 runes are chunked cleanly and delivered via `chat.postMessage`.

---

## 4. WhatsApp Adapter Setup (Meta Cloud API)

### Step 1: Set Up WhatsApp Cloud API
1. Navigate to the [Meta for Developers Portal](https://developers.facebook.com/).
2. Create a Business App and configure WhatsApp Cloud API.
3. Obtain your **Phone Number ID**, **Permanent System User Access Token** (`api_token`), and **App Secret**.
4. In Webhook configuration, set the Callback URL to `https://<your-domain>/whatsapp/webhook` and enter a secure **Verify Token**. Subscribe to the `messages` field.

### Step 2: Configure `config.yaml`
```yaml
gateway:
  enabled: true
  whatsapp:
    enabled: true
    api_token: "your-meta-bearer-token"
    phone_number_id: "your-phone-number-id"
    verify_token: "your-webhook-verify-token"
    app_secret: "your-meta-app-secret"
    listen_addr: ":3003"
    allowed_users:
      - "+15551234567"  # Allowed sender phone number
```

### Security & Media Handling:
- **Challenge Verification**: Handles `hub.challenge` handshake via constant-time token comparison.
- **HMAC Verification**: Enforces `X-Hub-Signature-256` SHA256 HMAC check.
- **Voice Notes**: Inbound voice messages/audio notes are automatically downloaded, bounded to 25MB, and transcribed via Whisper (`core.Transcriber`).
- **Chunking**: Messages over 4096 runes are chunked and delivered via Meta Graph API.

---

## Running the Gateway Daemon

Launch the daemon directly or via a systemd service:

```bash
# Start the gateway daemon with default config
./bin/agis gateway run

# Or specify a custom config file
./bin/agis gateway run --config /etc/agis/config.yaml
```

Output:
```text
2026/08/29 14:00:00 INFO gateway multiplexer: starting adapters count=2
2026/08/29 14:00:01 INFO telegram adapter: polling started username=@MyAgisBot
2026/08/29 14:00:01 INFO discord adapter: websocket connected to discord gateway
```

Press `Ctrl+C` or send `SIGTERM` to trigger clean teardown.

---

## Multimodal Ingestion in Gateways

When `multimodal.enabled: true`:

- **Telegram Photos & Voice Notes**:
  - Inbound photo messages are downloaded via `getFile` (highest resolution chosen) and forwarded to vision-capable models (`gpt-4o`, `llama3.2-vision`).
  - Inbound voice notes and audio clips are downloaded and transcribed via `core.Transcriber` (OpenAI Whisper), populating message content for the Brain.
- **Discord Attachments**:
  - Image attachments are downloaded from Discord's CDN and passed to the vision pipeline.
  - Audio attachments are downloaded, transcribed to text, and forwarded to the Brain.
- **Guardrails**:
  - Image size limit: 10MB (configurable via `multimodal.vision.max_image_size_mb`).
  - Audio size limit: 25MB (configurable via `multimodal.audio.max_audio_size_mb`).
  - MIME sniffing with `http.DetectContentType` to fail-closed on spoofed or executable files.

See [docs/multimodal.md](multimodal.md) for full configuration details.
