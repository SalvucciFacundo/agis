# Hermes Agent vs AGIS — Architectural Comparison & Parity Roadmap

Este documento detalla la investigación técnica sobre la arquitectura y capacidades de **Hermes Agent** (Nous Research), la comparativa exhaustiva con **AGIS** (Autonomous Go Intelligent System) y la hoja de ruta de implementación con Spec-Driven Development (SDD) y Strict TDD.

---

## 1. Anatomía Arquitectónica de Hermes Agent

Hermes Agent es un framework autónomo de propósito general con un loop de aprendizaje continuo y ejecución de herramientas, estructurado en seis pilares:

```
┌────────────────────────────────────────────────────────────────────────┐
│                        1. CORE & REASONING                             │
│   Brain Loop • Subagents (delegate_task) • Fallback • MoA • Prompts    │
├───────────────────────────────────┬────────────────────────────────────┤
│       2. MEMORIA & SKILLS         │       3. IDENTIDAD & PROFILES      │
│   MEMORY.md • USER.md • Curator   │   SOUL.md • Multi-Profiles (~/.h)  │
│   Skills System (agentskills.io)  │   Persona Overlays & Guidance      │
├───────────────────────────────────┴────────────────────────────────────┤
│                     4. TOOL SYSTEM & BACKENDS                          │
│   ~86 Built-in Tools (Toolsets) • Tool Search / On-demand schemas      │
│   Backends: Local, Docker, SSH, Modal, Daytona, Sandbox, Singularity   │
├───────────────────────────────────┬────────────────────────────────────┤
│     5. SUPERFICIES & GATEWAYS     │          6. OPERACIÓN & CLI        │
│   TUI • 20+ Chat Gateways • Cron  │   CLI Setup Wizard • Model/Tools   │
│   Webhooks • API Server (OpenAI)  │   Doctor • Config Accessors        │
└───────────────────────────────────┴────────────────────────────────────┘
```

---

## 2. Matriz de Estado y Paridad en AGIS

| Capacidad / Componente | Hermes Agent (Python) | AGIS (Go Actual) | Estado en AGIS |
|---|---|---|---|
| **Arquitectura de Software** | Monolito modular en Python. | Arquitectura Hexagonal estricta en Go puro. | 🚀 **AGIS Superior** |
| **Distribución y Runtime** | Runtime Python, venv, paquetes C/Python (~2.5GB). | Binario estático único sin CGO (`modernc.org/sqlite`, ~20MB). | 🚀 **AGIS Superior** |
| **Consumo de Memoria RAM** | **~1.2 GB a 2.1 GB** en reposo/producción. | **~15 MB a 35 MB** en reposo/producción. | 🚀 **AGIS Superior (98% ahorro)** |
| **Tiempo de Arranque (Cold Start)** | ~2.5 a 4.5 segundos. | **4 milisegundos (<0.005s)**. | 🚀 **AGIS Superior (1000x)** |
| **Persistencia de Memoria** | Archivos planos de texto (`MEMORY.md`, `USER.md`). | SQLite + FTS5 + Hybrid Vector Search (RRF). | 🚀 **AGIS Superior** |
| **Seguridad y Permisos** | Listas de control de acceso básicas. | Policy Guard fail-closed (`sandbox`, `standard`, `full`) + auditoría. | 🚀 **AGIS Superior** |
| **Búsqueda Web y Fetching** | Búsqueda web nativa + scraping. | `web_search` (Brave/Tavily/SearXNG/DDG) + `web_fetch` anti-SSRF. | ✅ **Fase 1 Shipped** |
| **Subagentes (`delegate_task`)** | Subagentes concurrentes aislados. | `subagents.Engine`, semáforo bounded, repo efímero en memoria. | ✅ **Fase 2 Shipped** |
| **Resiliencia LLM & Failover** | Cadenas de failover + Key pools + MoA. | `FallbackProvider` chains, `CredentialPool` rotación 429, streaming seguro. | ✅ **Fase 3 Shipped** |
| **Setup & Multi-Perfiles** | `hermes setup` + `hermes profile`. | `agis setup` interactivo/0600 + `agis profile` con `$AGIS_HOME` aislado. | ✅ **Fase 4 Shipped** |
| **Servidor API Compatible** | `/v1/chat/completions` para WebUIs. | `internal/server` (`agis serve`) + 11 presets de proveedores LLM. | ✅ **Fase 5 Shipped** |
| **Gateways de Mensajería** | 20+ plataformas (Telegram, Discord, etc.). | Telegram, Discord, Slack y WhatsApp nativos con audio Whisper. | ✅ **Fase 6 Shipped** |
| **Tool Search Dinámico** | Carga perezosa de herramientas. | `tool_search` + `load_tool` con poda de esquemas en `Brain.Step`. | ✅ **Fase 6 Shipped** |

---

## 3. Hoja de Ruta de Fases en AGIS

### Fases Completadas (Shipped to Main)
- [x] **Fase 1: Herramientas Nativas de Búsqueda y Web (`internal/tools/web`)** — `web_search` multi-motor + `web_fetch` seguro con extractor AST HTML-a-Markdown en Go puro.
- [x] **Fase 2: Delegación de Subagentes (`internal/subagents`, `delegate_task`)** — Repositorio efímero en memoria, semáforo de concurrencia, límites de recursión (profundidad 2) y síntesis de resultados.
- [x] **Fase 3: Resiliencia del Proveedor LLM y Fallback Providers (`internal/adapters/llm`)** — `FallbackProvider` compuesto, `CredentialPool` con rotación 429 anti-estampida, pre-token stream switching y overrides para tareas auxiliares.
- [x] **Fase 4: Experiencia de Onboarding y Multi-Perfiles (`cmd/agis/setup.go`, `cmd/agis/profile.go`)** — `agis setup / init` con probe de 5s y permisos 0600, y espacios multi-perfil bajo `$AGIS_HOME/profiles/<name>/` con flag global `--profile`.
- [x] **Fase 5: Servidor API Compatible con OpenAI y 11 Proveedores LLM (`internal/server`, `agis serve`)** — `POST /v1/chat/completions` (SSE streaming & sync), `/v1/models`, `/healthz`, Bearer auth, CORS y 11 presets (incluyendo cliente nativo Claude Anthropic `/v1/messages`).
- [x] **Fase 6: Gateways Adicionales (Slack y WhatsApp) y Tool Search Dinámico (`internal/gateway`, `internal/tools`)** — Adaptadores Slack Events API y WhatsApp Cloud API con validación HMAC en tiempo constante, audio Whisper, y herramientas `tool_search`/`load_tool` con poda de esquemas en el Brain.

---

### Próximas Fases Planificadas (Backlog de Arquitectura)

#### Fase 7: Skill Index por Triggers & Skill Creator (`internal/skills`, `cmd/agis`)
- **Índice Ligero en Prompt**: Inyectar únicamente la tabla de triggers de skills (`Name`, `Description`, `Trigger`) en el system prompt en lugar del contenido completo `SKILL.md`, reduciendo drásticamente el consumo de tokens.
- **Carga Perezosa (`read_skill`)**: Herramienta nativa que permite al LLM cargar el cuerpo completo de instrucciones de una skill solo cuando la tarea coincide con el trigger.
- **Skill Creator Nativo (`agis skill create <name>` / `create_skill` tool)**: Asistente y generador de skills con frontmatter YAML compatible con `agentskills.io` y Gentle AI (*When to Use*, *Critical Rules*, *Steps*, *Examples*).

#### Fase 8: Aprendizaje de Subagentes hacia la Memoria Persistente (`internal/subagents`, `internal/memory`)
- **Hook de Destilación de Conocimiento**: Antes de que el subagente efímero sea destruido en `subagents.Engine.Spawn`, un hook de extracción captura descubrimientos y decisiones clave (`type: discovery`, `decision`, `pattern`).
- **Persistencia en la Memoria del Padre**: Guarda las observaciones duraderas directamente en `agis.db` (`observations` con `topic_key`), permitiendo que el cerebro principal recuerde para siempre lo que investigaron sus subagentes.

#### Fase 9: Lazy MCP Spawning / Standby (`internal/mcp`)
- **Arranque Bajo Demanda**: Los servidores MCP configurados permanecen en estado `Standby`. AGIS conoce sus metadatos y esquemas de herramientas, pero **no inicia los subprocesos `stdio` ni abre conexiones SSE** hasta el instante exacto en que el LLM invoca una herramienta `mcp:<server>:<tool>`.
- **Auto-Shutdown por Inactividad**: Apagado automático de procesos MCP tras X minutos de inactividad, logrando 0% de uso de RAM en reposo.

#### Fase 10: Portabilidad de Perfiles (`agis backup` / `agis restore`)
- **Exportación en un Comando**: `agis backup [perfil]` genera un archivo empaquetado `.tar.gz` comprimido con toda la configuración, base SQLite `agis.db`, `SOUL.md`, `skills/` y `policy.yaml`.
- **Importación y Migración**: `agis restore <tarball> [--profile <nombre>]` descomprime y valida la integridad de un perfil para migrarlo entre máquinas o servidores en segundos.

#### Fase 11: Text-to-Speech (TTS) Saliente (`internal/adapters/audio`)
- **Sintetizador de Voz**: Adaptador de salida para Text-to-Speech (ej. OpenAI TTS, ElevenLabs, Kokoro local).
- **Respuestas en Audio**: Permite que los gateways de Telegram y WhatsApp respondan con notas de voz sintetizadas cuando el usuario se comunica por audio.

#### Fase 12: Browser Automation Headless con Playwright/Chromium (`internal/tools/browser`)
- **Automatización Web Avanzada**: Complemento opcional para sitios 100% Single Page Applications (SPAs) donde se requiere renderizado completo de JavaScript, navegación, clicks y capturas de pantalla.

---

## 4. Arquitectura de Workspaces Multi-Agente (Perfiles Autónomos)

AGIS soporta la creación de **agentes completamente autónomos y especializados** mediante su sistema de perfiles:

```
~/.agis/profiles/
├── default/              # Perfil generalista base
├── coder/                # Especialista en Desarrollo & Arquitectura
│   ├── SOUL.md           # Identidad de Senior Architect
│   ├── config.yaml       # Modelos Claude/DeepSeek, MCP GitHub/Postgres
│   ├── skills/           # go-testing, refactoring, sql-opt
│   └── agis.db           # Memoria de código
│
├── support-telegram/     # Bot Autónomo de Atención al Cliente
│   ├── SOUL.md           # Identidad de Soporte
│   ├── config.yaml       # Gateway Telegram Bot A, Modelo local/rápido
│   ├── skills/           # faq, escalation
│   └── agis.db           # Memoria de tickets y usuarios
│
└── sales-whatsapp/       # Bot Autónomo Comercial
    ├── SOUL.md           # Identidad de Ventas
    ├── config.yaml       # Gateway WhatsApp Bot B
    └── agis.db           # Memoria de leads
```

- **Independencia Total**: Cada perfil/agente tiene su propio método de conexión (bots de Telegram distintos, números de WhatsApp distintos, canales de Slack distintos), su propia personalidad, sus propios modelos, sus propias skills y su propia base de datos SQLite.
- **A discreción del usuario**: AGIS inicia con el perfil básico `default`, y el usuario crea y personaliza nuevos perfiles según sus necesidades (`agis profile create <name>`).
