# harness

Собственный AI agent harness на Go.

- Язык: Go 1.26+
- TUI: Bubbletea (в следующих milestone)
- Провайдеры: OpenAI-совместимый (ChatGPT/DeepSeek/OpenRouter), Gemini, Codex через прокси подписки
- Ключевые фичи: dual-brain (thinker/worker), hashline edit, failover-роутер, сабагенты

## Документация

Полное ТЗ и архитектура — [`HANDOFF.md`](HANDOFF.md).
Инструкции для агентов — [`AGENTS.md`](AGENTS.md).

## Быстрый старт

```powershell
go build ./...
copy config.example.json config.json
$env:OPENAI_API_KEY = "..."
go run ./cmd/harness -config config.json
```

## Статус

- [x] Milestone 0 — скелет (конфиг, интерфейсы, структура)
- [ ] Milestone 1 — OpenAI-совместимый провайдер + read_file/shell
- [ ] Milestone 2 — tool calls и агентный цикл
- [ ] Milestone 3 — hashline edit, grep, glob
- [ ] Milestone 4 — Gemini + роутер
- [ ] Milestone 5 — TUI
- [ ] Milestone 6 — сессии и сабагенты
