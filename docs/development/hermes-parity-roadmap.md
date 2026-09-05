# Hermes Agent vs AGIS — Architectural Comparison & Parity Roadmap

Este documento detalla la investigación técnica sobre la arquitectura y capacidades de **Hermes Agent** (Nous Research), la comparativa exhaustiva con **AGIS** (Autonomous Go Intelligent System) y la hoja de ruta para alcanzar la paridad funcional con máxima eficiencia de recursos en Go.

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

### 1.1 Core Reasoning Loop, Subagentes y Resiliencia
- **Loop Cognitivo Bounded**: Ejecución multi-ronda de llamados a herramientas (tool calls) evaluados bajo límites configurables.
- **Subagent Delegation (`delegate_task`)**: Instanciación de agentes hijos con contextos aislados, terminales dedicadas y toolsets restringidos. Los subagentes resuelven subtareas pesadas en paralelo y devuelven un resumen conciso al agente padre, protegiendo la ventana de contexto.
- **Tolerancia a Fallos y Resiliencia Multi-Capa**:
  1. *Pool de Credenciales*: Rotación automática de API keys ante rate limits (HTTP 429).
  2. *Fallback de Proveedor*: Conmutación en caliente hacia modelos secundarios cuando el principal falla (5xx, timeouts).
  3. *Auxiliary Task Overrides*: Resolución de modelos independientes para tareas específicas (visión, transcripción de audio, web extraction).
- **Mixture of Agents (MoA)**: Proveedor virtual que consulta múltiples modelos en paralelo y sintetiza respuestas a través de un modelo agregador.

### 1.2 Memoria y Aprendizaje Continuo
- **Memoria Curada Persistente**:
  - `MEMORY.md`: Notas operativas y conocimiento duradero acumulado (~2.200 caracteres).
  - `USER.md`: Perfil, preferencias y hechos del usuario (~1.375 caracteres).
- **Skills Procedimentales**: Cumplimiento con la especificación `agentskills.io` (Markdown con frontmatter). Soporta carga dinámica, edición y destilación de nuevas skills en runtime (`source: agent`).
- **Loop de Cierre de Sesión**: Extracción de observaciones y resumen automático.

### 1.3 Identidad, Personalidad y Perfiles
- **`SOUL.md`**: Definición de la personalidad base e identidad del agente en el Nivel 1 del system prompt.
- **Aislamiento Multi-Perfil (`hermes profile`)**: Espacios de trabajo completamente independientes (`~/.hermes/profiles/<nombre>`), cada uno con su propio almacenamiento SQLite, memorias, skills y configuración.

### 1.4 Catálogo de Herramientas y Backends
- **~86 Herramientas Agrupadas en Toolsets**:
  - *File*: `read_file`, `write_file`, `edit_file`, `list_dir`.
  - *Terminal*: `execute_command`, `read_terminal`.
  - *Web & Search*: Búsqueda web (Tavily, Serper, Jina, Brave), web scraping y parsing a Markdown.
  - *Browser*: Automatización con Playwright / CDP (navegación, capturas, clicks, extracción DOM).
  - *Code Execution*: Intérprete Python interactivo en sandbox.
  - *Multimodal*: Visión, Text-to-Speech (TTS) y Whisper Audio Transcription.
  - *Integraciones*: Home Assistant, Desktop GUI automation.
- **Tool Search / Lazy Tool Loading**: En lugar de inyectar decenas de esquemas JSON en el prompt, provee herramientas puente para buscar y cargar dinámicamente esquemas bajo demanda.
- **Backends de Ejecución**: Local Shell, Docker, SSH, Modal, Daytona, Vercel Sandbox, Singularity/Apptainer.

### 1.5 Superficies y Servidor de Integración
- **Messaging Gateway (20+ plataformas)**: Telegram, Discord, Slack, WhatsApp (Baileys web bridge), Signal, Teams, SMS, Email, etc.
- **Cron Scheduler & Webhooks**: Ejecución de rutinas desatendidas y receptor de eventos HTTP con verificación HMAC-SHA256.
- **OpenAI-Compatible REST API Server (`hermes serve`)**: Expone un endpoint `/v1/chat/completions` para conectar UIs web (Open WebUI, LibreChat, LobeChat).

### 1.6 Experiencia de Operación y CLI
- TUI interactiva con autocompletado y comandos slash.
- Comandos de gestión: `setup`, `model`, `tools`, `profile`, `doctor`, `update`, `config get/set`.

---

## 2. Matriz Comparativa: Hermes Agent vs AGIS

| Capacidad / Componente | Hermes Agent (Python) | AGIS (Go Actual) | Veredicto Arquitectónico |
|---|---|---|---|
| **Arquitectura de Software** | Monolito modular en Python. | Arquitectura Hexagonal estricta (Puertos y Adaptadores) en Go puro. | 🚀 **AGIS**: Mayor desacoplamiento, testabilidad y tipos estrictos. |
| **Distribución y Runtime** | Runtime de Python, venv, múltiples paquetes C/Python. | Binario estático único sin CGO (`modernc.org/sqlite`). | 🚀 **AGIS**: Cero dependencias externas, despliegue trivial. |
| **Consumo de Memoria y CPU** | ~180MB - 450MB en reposo; arranque lento (~1-3s). | ~15MB - 30MB en reposo; arranque instantáneo (<50ms). | 🚀 **AGIS**: ~10x a 15x más eficiente en memoria. |
| **Motor de Persistencia** | Archivos planos de texto (`MEMORY.md`, `USER.md`). | SQLite + FTS5 + Hybrid Search (RRF con Embeddings vectoriales). | 🚀 **AGIS**: Búsquedas semánticas y léxicas escalables y transaccionales. |
| **Seguridad y Permisos** | Listas de control de acceso básicas. | Policy Guard fail-closed (`sandbox`, `standard`, `full`) + auditoría completa. | 🚀 **AGIS**: Modelo de seguridad multi-nivel de nivel empresarial. |
| **Identidad y Skills** | `SOUL.md` + `agentskills.io` Markdown. | `SOUL.md` + `agentskills.io` + destilación runtime. | ✅ **Paridad Completa**. |
| **TUI y Comandos CLI** | Bubbletea/PromptToolkit, comandos de config. | Bubbletea TUI + `doctor`, `session`, `update`, `config`, `policy`. | ✅ **Paridad Completa**. |
| **MCP Client** | JSON-RPC 2.0 (stdio/SSE). | JSON-RPC 2.0 nativo en Go puro con process groups y streams SSE. | ✅ **Paridad Completa**. |
| **Chat Gateways** | 20+ plataformas de mensajería. | Telegram y Discord nativos (multiplexados sobre el mismo Brain). | 🟡 **Parcial**: Faltan adaptadores para Slack, WhatsApp, Signal. |
| **Multimodalidad** | Visión + TTS + Whisper. | Visión Data URLs + Whisper STT. | 🟡 **Parcial**: Falta motor TTS. |
| **Subagentes (`delegate_task`)** | Subagentes concurrentes aislados. | No implementado. | 🔴 **Falta en AGIS**. |
| **Tolerancia a Fallos (Fallback/MoA)** | Cadenas de failover + Key pools + MoA. | 1 proveedor activo a la vez. | 🔴 **Falta en AGIS**. |
| **Búsqueda Web y Scraper Nativo** | Búsqueda web (Tavily/Brave/Jina) + scraping. | Requiere servidor MCP externo. | 🔴 **Falta en AGIS**. |
| **Tool Search (Lazy Loading)** | Inyección perezosa de herramientas. | Inyección estática de todas las tools registradas. | 🔴 **Falta en AGIS**. |
| **Aislamiento Multi-Perfil** | `hermes profile` con `$HOME` aislado. | Perfil único en `$AGIS_HOME`. | 🔴 **Falta en AGIS**. |
| **Servidor API Compatible** | Servidor HTTP `/v1/chat/completions`. | No implementado. | 🔴 **Falta en AGIS**. |
| **Wizard de Onboarding** | `hermes setup` interactivo guiado. | No implementado (config manual por flags). | 🔴 **Falta en AGIS**. |

---

## 3. Hoja de Ruta para AGIS (Paridad Funcional + Superioridad de Rendimiento)

Para convertir a AGIS en el reemplazo definitivo de Hermes manteniendo su filosofía de eficiencia extrema y cero dependencias:

### Fase 1: Herramientas Nativas de Búsqueda y Web (`internal/tools/web`)
- Implementar clientes ligeros en Go para búsqueda web (Brave Search API, Tavily, DuckDuckGo HTML scraping, SearXNG).
- Implementar extractor de contenido web limpio a Markdown (Readability / HTML tokenizer en Go puro).
- Registrar las tools `web_search` y `web_fetch` en el `tools.Registry` nativo.

### Fase 2: Delegación de Subagentes (`internal/core/subagents`)
- Crear herramienta nativa `delegate_task` en el Brain loop.
- Instanciar un `core.Brain` hijo con repositorio temporal o compartido, historial efímero y pool acotado de tools.
- Ejecutar subagentes en goroutines independientes con timeout y compresión automática del resultado hacia el contexto del padre.

### Fase 3: Resiliencia del Proveedor LLM y Fallback (`internal/adapters/llm`)
- Implementar cadena de proveedores con fallback automático (`FallbackProvider`).
- Soporte para rotación de múltiples API keys por proveedor.
- Configuración de overrides de modelos para tareas auxiliares (embeddings, transcripción, visión).

### Fase 4: Experiencia de Onboarding y Multi-Perfiles (`cmd/agis`)
- `agis setup` / `agis init`: Asistente interactivo en TUI para selección de proveedor, test de conectividad y generación de `config.yaml`.
- `agis profile [list|create|switch|delete]`: Soporte de múltiples perfiles bajo `$AGIS_HOME/profiles/<perfil>`.

### Fase 5: Servidor API Compatible con OpenAI (`internal/server`)
- `agis serve`: Servidor HTTP ligero exponiendo `/v1/chat/completions` y `/v1/models`.
- Compatibilidad con frontends web (Open WebUI, LibreChat, Chatbox).

### Fase 6: Gateways Adicionales y Tool Search Dinámico
- Adaptadores de mensajería para Slack y WhatsApp.
- Mecanismo de Tool Search para cargar esquemas JSON de herramientas bajo demanda cuando el número de herramientas supera el umbral del prompt.
