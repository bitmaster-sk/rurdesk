---
title: User Guide
description: Install, configure and operate the AI-native work management platform.
---

# User Guide

This guide covers everything you need to run the platform yourself: installing
the stack, configuring it, deploying an agent gateway, and operating the
day-to-day features.

## Who this is for

- **Administrators** setting up the stack, AI providers, and agent gateways
- **Team members** using tasks, views, and AI features day to day

## How the pieces fit together

The platform is a single Docker Compose stack plus an optional **agent gateway**:

| Component | Role |
| --- | --- |
| **Tracker** (`rurdesk`) | The whole web stack in **one image**: the Go binary serves the REST API, WebSocket, the **MCP server** and the Angular SPA on a single port, and calls the **AI provider** that powers quality check / kickstart / split |
| **Gateway** (optional, ×N) | Runs an **AI coding agent** (Goose or Claude Code) against your repos and reports back — add one container per agent |
| **PostgreSQL** | Primary datastore — **best run outside Docker** (managed service or host install); see [Installation](./installation.md) |
| **Redis** | Sessions, caching, real-time fan-out — same recommendation as Postgres |

The tracker is the only service published to the host (port **80** → container
`:1000`). TLS is terminated by your own load balancer or ingress in front of it —
see [Installation → Prerequisites](./installation.md). The separate
proxy / API / client containers you may see in the repository's
`docker-compose.yml` exist **only for local development**.

> **Two different "AI" paths — don't confuse them:**
>
> - **AI Quality Check, Project Kickstarter, Task Split** run **inside the API**
>   using an AI provider you configure (`AI_PROVIDER`, `AI_API_KEY`, …). These
>   need an AI key — or a local **Ollama** for no cloud keys at all.
> - **The Agent workflow** (brainstorm → design → plan → implement → PR) runs in
>   the **gateway**, which drives the **Goose** agent. Point it at a **local
>   Ollama** for a zero-key, fully self-hosted setup, or bring your own key —
>   **Ollama Cloud / Anthropic / Gemini**.

![Architecture](../../site/assets/img/architecture.svg)

## Contents

1. [Installation](./installation.md) — bring the stack up with Docker Compose, first run
2. [Configuration](./configuration.md) — environment variables, AI provider, database, cache
3. [User management](./user-management.md) — accounts, admins, bots, API keys, project roles
4. [Features](./features.md) — tasks, views, relations, tracker, and the AI features
5. [Agents](./agents.md) — add a bot, wire a gateway, run the agent workflow
6. [Agent Gateway](./gateway.md) — deploy and configure the Goose coding-agent gateway
7. [Git integration](./git-integration.md) — connect repositories, PR/MR diffs inline

## Requirements

- **Docker** and **Docker Compose**
- A git remote your agents can push to (for the agent workflow), plus a token with read/write
- One of: an **AI provider key** (Anthropic / OpenAI / Gemini) **or** a local **Ollama** install — for in-API AI features
- For the agent gateway (Goose): a **local Ollama** (no key) **or** a provider
  API key — **Ollama Cloud / Anthropic / Gemini**
