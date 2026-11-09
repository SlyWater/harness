# AGENTS.md — Go Harness

Этот проект — собственный AI agent harness на Go.

**Перед любой работой прочитай `docs/HANDOFF.md`** (или `HANDOFF.md`) —
там полное ТЗ: архитектура, интерфейсы, провайдеры, инструменты, роадмап.

Коротко:
- Язык: Go 1.26+, TUI на Bubbletea.
- Провайдеры: OpenAI-совместимый (ChatGPT/Codex, DeepSeek, OpenRouter) + Gemini.
- Ключевые фичи: dual-brain (thinker/worker), hashline edit, failover роутер, сабагенты.
- Конфиг: JSON, ключи только из env.
- Аккаунты: `internal/accounts` — индекс `~/.harness/accounts.json` + AES-256-GCM
  детали, ротация и кулдауны, CLI `harness accounts ...`.
- Не тащить лишние зависимости. `gofmt` + `go vet` обязательны.
