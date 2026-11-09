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
go run ./cmd/harness -config config.json "прочитай README.md"
```

## Аккаунты (мультиаккаунт)

```powershell
go run ./cmd/harness accounts add -provider deepseek -label work -key-env DEEPSEEK_KEY_1
go run ./cmd/harness accounts list
go run ./cmd/harness accounts use <id>
```

Хранилище: `~/.harness` (переопределяется `HARNESS_HOME`). Детали аккаунтов
шифруются AES-256-GCM, индекс `accounts.json` — без секретов.

## Статус

- [x] Milestone 0 — скелет (конфиг, интерфейсы, структура)
- [x] Milestone 1 — OpenAI-совместимый провайдер, read_file/shell, headless
- [x] Milestone 2.5 — система аккаунтов (шифрование, ротация, CLI)
- [ ] Milestone 2 — write_file и валидация JSON Schema
- [ ] Milestone 3 — hashline edit, grep, glob
- [ ] Milestone 4 — Gemini + роутер
- [ ] Milestone 5 — TUI
- [ ] Milestone 6 — сессии и сабагенты
