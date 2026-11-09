# Go Harness — Handoff / Техническое задание

> Документ для передачи проекта другому агенту (или разработчику).
> Прочитай целиком перед тем, как писать код. Если что-то противоречит —
> сначала спроси, потом меняй архитектуру.

---

## 1. Резюме проекта

Строим **собственный AI agent harness** (движок автономного кодинг-агента,
аналог OpenCode / OMP / DeepSeek Harness), но:

- **на Go** — минимальное потребление памяти, один бинарник, мгновенный старт;
- с **единым интерфейсом провайдеров**: ChatGPT / Codex (OpenAI), Google Gemini,
  DeepSeek и любой OpenAI-совместимый endpoint (OpenRouter, Together, локальный vLLM);
- с **двухмодельной схемой** «Мозг + Руки» (Thinker + Worker);
- с **надёжным редактированием кода** (hashline / fuzzy edit), а не слепой перезаписью файлов;
- с **отказоустойчивой маршрутизацией** (ротация ключей, фоллбэк при 429/502);
- с **TUI** на Bubbletea и поддержкой **сабагентов**.

Рабочая директория проекта: `C:\Users\slywater\projects\Harness`.

---

## 2. Цели и не-цели

### Цели (MVP)
1. Один агентный ReAct-цикл: `LLM → tool calls → результаты → LLM → ... → ответ`.
2. Стриминг: reasoning/thinking токены, обычный текст, tool-call чанки.
3. Инструменты: `read_file`, `write_file`, `hashline_edit`, `shell`, `grep`, `glob`.
4. Провайдеры: OpenAI-совместимый (ChatGPT, DeepSeek, OpenRouter) + Gemini.
5. Конфиг из файла (`config.toml` / `config.json`) + переменные окружения.
6. TUI-чат с историей, статусом инструментов и индикатором стрима.
7. Сохранение/загрузка сессий на диск.
8. Ротация ключей и фоллбэк между провайдерами.

### Не-цели (пока)
- Веб-интерфейс и мобильное приложение.
- Полноценный plugin-фреймворк уровня Cordis (не повторяем ошибки DSH).
- Поддержка MCP (закладываем интерфейс, но не реализуем в MVP).
- Git-worktree-оркестрация (позже).
- Мультиплатформенная упаковка/инсталляторы.

---

## 3. Стек и обоснование

| Компонент | Решение | Почему |
|---|---|---|
| Язык | **Go 1.26+** | быстрый старт, малый RAM, простые горутины |
| TUI | **Bubbletea + Lipgloss + Bubbles** | эталон терминального UI, удобные компоненты |
| HTTP | `net/http` + `encoding/json` | без тяжёлых SDK, полный контроль над стримом |
| Конфиг | `encoding/json` (стандартная библиотека) | без лишних зависимостей |
| Логи | `log/slog` | стандартная библиотека |
| Тесты | `testing` + `httptest` | стандартная библиотека |

**Правило:** минимум внешних зависимостей. Каждая новая зависимость должна
быть обоснована в PR/коммите.

---

## 4. Архитектура

```
┌──────────────────────────────────────────────────────────────┐
│                      Интерфейсы (adapters)                   │
│        TUI (Bubbletea)   │   headless/CLI   │   (позже: RPC)  │
└──────────────────────────┬───────────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────────┐
│                     Harness Core (engine)                    │
│  - Agent Loop (ReAct)                                        │
│  - Session / History Manager                                 │
│  - Event Bus / Hooks (on_token, on_tool_start, on_tool_end)  │
│  - Router (failover, key rotation, role→model)               │
└───────────────┬──────────────────────────────┬───────────────┘
                │                              │
┌───────────────▼──────────────┐ ┌─────────────▼───────────────┐
│      Providers (llm)         │ │       Tools (tools)         │
│  - openai_compatible         │ │  - read_file                │
│  - gemini                    │ │  - write_file               │
│  - (codex через openai)      │ │  - hashline_edit            │
│  - mock (для тестов)         │ │  - shell                    │
└──────────────────────────────┘ │  - grep / glob              │
                                 └─────────────────────────────┘
```

Поток данных:
1. UI отправляет `user message` в `Engine`.
2. `Engine` собирает `Request` (system prompt + история + tool schemas).
3. `Router` выбирает провайдера/модель/ключ и вызывает `Provider.Stream`.
4. Стрим токенов уходит в Event Bus → UI обновляется в реальном времени.
5. Если пришёл `tool_call`, `Engine` валидирует аргументы, вызывает `Tool.Run`.
6. Результат добавляется в историю как `tool` сообщение, цикл повторяется.
7. Выход: нет tool calls, или лимит шагов/токенов, или отмена пользователем.

---

## 5. Структура репозитория

```
Harness/
├── cmd/
│   └── harness/
│       └── main.go            # точка входа, парсинг флагов, запуск TUI/headless
├── internal/
│   ├── config/
│   │   └── config.go          # загрузка и валидация конфига
│   ├── llm/
│   │   ├── provider.go        # интерфейсы Provider, StreamEvent, ToolSchema
│   │   ├── openai.go          # OpenAI-совместимый стрим (SSE)
│   │   ├── gemini.go          # Gemini generateContent stream
│   │   ├── mock.go            # фейковый провайдер для тестов
│   │   └── router.go          # выбор модели/ключа, failover, ротация
│   ├── engine/
│   │   ├── engine.go          # agent loop
│   │   ├── session.go         # история, сериализация
│   │   └── events.go          # типы событий Event Bus
│   ├── tools/
│   │   ├── tool.go            # интерфейс Tool + реестр
│   │   ├── read_file.go
│   │   ├── write_file.go
│   │   ├── hashline.go        # hashline/fuzzy edit
│   │   ├── shell.go           # запуск команд, таймауты, усечение вывода
│   │   ├── grep.go            # поиск по содержимому (чистый Go или ripgrep)
│   │   └── glob.go
│   ├── prompt/
│   │   └── system.go          # сборка системного промпта
│   └── tui/
│       ├── model.go           # Bubbletea model
│       ├── view.go            # рендер
│       └── components.go      # индикаторы, список тулов, диффы
├── docs/
│   └── HANDOFF.md             # этот файл
├── config.example.json
├── go.mod
├── go.sum
└── README.md
```

---

## 6. Ключевые интерфейсы (черновик кода)

### 6.1 Провайдер

```go
package llm

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"` // всегда валидный JSON
}

type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

type Request struct {
	Model       string
	System      string
	Messages    []Message
	Tools       []ToolSchema
	Temperature float64
	MaxTokens   int
}

// StreamEvent — единый формат для всех провайдеров.
type StreamEvent struct {
	Type      EventType
	Text      string     // для TypeText / TypeThinking
	ToolCall  *ToolCall  // для TypeToolCall
	Usage     *Usage     // для TypeDone (если провайдер отдаёт)
	Err       error      // для TypeError
}

type EventType int

const (
	EventText EventType = iota
	EventThinking
	EventToolCall
	EventDone
	EventError
)

type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

type Provider interface {
	Name() string
	// Stream отправляет события в канал. Закрывает канал по завершении.
	// Отмена через ctx должна немедленно прекращать чтение HTTP-потока.
	Stream(ctx context.Context, req Request) (<-chan StreamEvent, error)
}
```

### 6.2 Инструмент

```go
package tools

import "context"

type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON Schema параметров
	// Run получает сырой JSON аргументов, возвращает текст результата.
	// Ошибки выполнения должны возвращаться как результат (is_error), а не паника.
	Run(ctx context.Context, args json.RawMessage) (Result, error)
}

type Result struct {
	Output  string
	IsError bool
}
```

### 6.3 События движка (для UI)

```go
package engine

type Event struct {
	Kind     EventKind
	Text     string
	ToolName string
	ToolArgs string
	ToolOut  string
	Err      error
}

type EventKind int

const (
	EvToken EventKind = iota
	EvThinking
	EvToolStart
	EvToolEnd
	EvDone
	EvError
)
```

---

## 7. Провайдеры

### 7.1 OpenAI-совместимый (`openai.go`)
Используется для: **ChatGPT (gpt-4o / o3)**, **DeepSeek (deepseek-chat / deepseek-reasoner)**,
**OpenRouter**, **Together**, локальный **vLLM/Ollama**.

- Endpoint: `POST {base_url}/chat/completions`, `"stream": true`.
- Формат SSE: строки `data: {...}`, завершение `data: [DONE]`.
- В дельтах читать:
  - `choices[0].delta.content` → `EventText`;
  - `choices[0].delta.reasoning_content` (DeepSeek R1) или `reasoning` (OpenRouter) → `EventThinking`;
  - `choices[0].delta.tool_calls[]` → накапливать по `index`, собрать `id/name/arguments`,
    отдать `EventToolCall` когда аргументы закрылись;
  - `choices[0].finish_reason` → завершение.
- Заголовки: `Authorization: Bearer <key>`.
- Codex: используется **подписка ChatGPT через прокси** — это отдельный адаптер,
  см. 7.4. Не путать с обычным OpenAI API по ключу.

### 7.2 Gemini (`gemini.go`)
- Endpoint: `POST https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse&key={key}`.
- Роли: `user` / `model`; system instruction передаётся отдельным полем `systemInstruction`.
- Tool calls: `functionCall { name, args }` внутри `parts`.
- Результаты инструментов: `functionResponse { name, response }`.
- Thinking: учитывать `thinkingConfig` / поля reasoning, если модель их отдаёт.

### 7.3 Router (`router.go`)
Отвечает за:
- выбор модели по роли: `thinker` (планирование) vs `worker` (исполнение тулов);
- пул ключей на провайдера и их ротацию при `401/429`;
  ключи берутся из `api_key_env` и `api_key_envs` (значение может быть списком
  через запятую), см. `ProviderConfig.ResolveKeys()`;
- фоллбэк: `deepseek → openrouter → gemini`, если провайдер вернул `5xx` или таймаут;
- логирование, какой ключ/провайдер реально ответил.

Правило: роутер **не меняет** формат `Message` — он только выбирает `Provider`
и подставляет модель/ключ.

### 7.4 Codex через прокси подписки (`codex.go`)

Отдельный адаптер для **подписки ChatGPT**, а не для pay-per-token API.

- Конфиг задаёт `type = "codex_proxy"` и `base_url` локального/удалённого прокси,
  который транслирует запросы к подписке в OpenAI-совместимый формат.
- Транспорт и формат стрима — как в `openai.go` (SSE). Отличие только в
  авторизации (не `Bearer <api_key>`, а токен/сессия прокси) и в `base_url`.
- Если прокси отдаёт нестандартные поля — нормализовать в `StreamEvent` здесь,
  не размазывать по ядру.
- Прокси не входит в репозиторий: harness только клиент, адрес задаётся в конфиге.
- Ошибки авторизации прокси (401/403) — отдельный тип, чтобы роутер не крутил
  ключи, а сразу сообщал пользователю «обнови сессию прокси».

---

## 8. Инструменты

### 8.1 `read_file`
- Параметры: `path`, `offset` (строка, 1-based), `limit`.
- Возвращает содержимое с номерами строк (`123: текст`) — это нужно для hashline edit.
- Защита: не читать файлы больше лимита (например, 2 МБ), обрезать с пометкой.

### 8.2 `write_file`
- Полная перезапись — только для новых файлов или когда модель явно так просит.
- В системном промпте предупреждать: для правок существующих файлов использовать `hashline_edit`.

### 8.3 `hashline_edit` (важнейший инструмент)
Цель: точечное редактирование без перезаписи файла и без поломки кода.

Формат, который видит модель (пример):
```
12:a3f1 | func main() {
13:9c02 |     fmt.Println("hi")
14:77bd | }
```
- `read_file` возвращает строки с префиксом `<line>:<hash> | <content>`.
- `hash` — короткий хэш содержимого строки (например, первые 4 hex от xxhash/sha256 строки).
- Модель вызывает `hashline_edit` с операциями:
  ```json
  {
    "path": "main.go",
    "edits": [
      {"start_line": 13, "end_line": 13, "expected_hash": "9c02", "replacement": "    fmt.Println(\"bye\")"}
    ]
  }
  ```
- Валидация:
  1. Для каждой правки проверить, что `hash` строки совпадает с `expected_hash`.
  2. Если не совпал — вернуть ошибку с актуальными строками и попросить модель перечитать файл.
  3. Применять правки **снизу вверх**, чтобы не сбить номера строк.
- Поддерживать вставку (`replacement` без удаления), удаление (`replacement: ""`).

> Это ключевое отличие от «перепиши файл целиком». Не упрощать до `str_replace`.

### 8.4 `shell`
- Запуск через `os/exec` с `context.WithTimeout`.
- Windows: `powershell -NoProfile -Command <cmd>`; Unix: `sh -c`.
- Возвращать `stdout`, `stderr`, `exit code`.
- Усекать вывод (например, первые/последние N КБ) с пометкой `[truncated]`.
- Опасные команды (`rm -rf`, `git reset --hard`, `format`) — помечать и (позже) спрашивать подтверждение.

### 8.5 `grep`
- Поиск по содержимому файлов. Приоритет — **`ripgrep` (`rg`), если он есть в PATH**.
- Если `rg` не найден — fallback на встроенный обход дерева + `regexp` (чистый Go).
- Параметры: `pattern`, `path`, `include` (glob), `case_sensitive`, `limit`.
- Возвращать `path:line: text`.
- Факт использования `rg` vs fallback логировать (`slog.Debug`).

### 8.6 `glob`
- Поиск файлов по маске (`**/*.go`), с игнором `.git`, `node_modules`, бинарников.

---

## 9. Агентный цикл (`engine.go`)

Псевдокод:

```
history = [user message]
for step in 1..max_steps:
    req = Request{
        Model:    router.ModelFor(role),
        System:   prompt.System(tools),
        Messages: history,
        Tools:    schemas,
    }
    events = provider.Stream(ctx, req)

    assistantMsg = Message{Role: assistant}
    for ev in events:
        switch ev.Type:
            Text:      ui.Send(EvToken, ev.Text); assistantMsg.Content += ev.Text
            Thinking:  ui.Send(EvThinking, ev.Text)
            ToolCall:  assistantMsg.ToolCalls = append(..., ev.ToolCall)
            Done:      break
            Error:     handle / fallback

    history = append(history, assistantMsg)

    if len(assistantMsg.ToolCalls) == 0:
        return assistantMsg.Content   // готово

    for tc in assistantMsg.ToolCalls:
        ui.Send(EvToolStart, tc.Name, tc.Arguments)
        result = tools.Run(ctx, tc.Name, tc.Arguments)
        ui.Send(EvToolEnd, result)
        history = append(history, Message{Role: tool, ToolCallID: tc.ID, Content: result})
```

Правила:
- Жёсткий лимит шагов (`max_steps`, по умолчанию 25) — защита от бесконечных циклов.
- Отмена через `context.Context` (Ctrl+C в TUI) должна прерывать и HTTP-стрим, и shell.
- Ошибка инструмента НЕ должна валить цикл — она уходит модели как `tool` сообщение с текстом ошибки.

---

## 10. Сабагенты (после MVP)

- Интерфейс `Agent` с методом `Run(ctx, task) (result, error)`.
- Сабагент = отдельная сессия с собственным набором тулов и моделью (обычно worker).
- Родитель получает только финальный результат, а не весь внутренний контекст.
- Запуск конкурентно через горутины; агрегация результатов.
- Позже: «совет» из N моделей (map-reduce) для ресёрча.

---

## 11. Конфиг (`config.example.json`)

```json
{
  "defaults": {
    "thinker": "openai:gpt-4o",
    "worker": "deepseek:deepseek-chat",
    "max_steps": 25,
    "max_tokens": 8192,
    "temperature": 0.2
  },
  "providers": {
    "openai": {
      "type": "openai_compatible",
      "base_url": "https://api.openai.com/v1",
      "api_key_env": "OPENAI_API_KEY"
    },
    "deepseek": {
      "type": "openai_compatible",
      "base_url": "https://api.deepseek.com/v1",
      "api_key_env": "DEEPSEEK_API_KEY"
    },
    "openrouter": {
      "type": "openai_compatible",
      "base_url": "https://openrouter.ai/api/v1",
      "api_key_env": "OPENROUTER_API_KEY"
    },
    "gemini": {
      "type": "gemini",
      "base_url": "https://generativelanguage.googleapis.com",
      "api_key_env": "GEMINI_API_KEY"
    },
    "codex": {
      "type": "codex_proxy",
      "base_url": "http://127.0.0.1:8787/v1",
      "api_key_env": "CODEX_PROXY_TOKEN"
    }
  },
  "router": {
    "fallback": ["deepseek", "openrouter", "gemini"]
  }
}
```

Ключи **никогда** не хранить в git. Только через env (`api_key_env`).
В JSON нет комментариев — все пояснения держать в этом документе.

---

## 12. TUI (Bubbletea)

MVP-экран:
- Верх: история чата (сообщения user/assistant, thinking — серым, тулы — компактно).
- Во время стрима: спиннер + живой текст.
- Показ вызова тула: `⚙ shell: go test ./...` → затем `✓ / ✗`.
- Низ: input-поле (textarea), Enter — отправить, Ctrl+C — отмена/выход.
- Сайдбар/статус: активная модель, шаг, токены (если есть usage).

Не переусложнять: сначала рабочий чат, потом красивости.

---

## 13. Роадмап

### Milestone 0 — скелет ✅
- [x] `go mod init`
- [x] структура папок
- [x] `config.go` + пример конфига (JSON)
- [x] интерфейсы `Provider`, `Tool`, `Message`
- [x] `ParseModelRef("provider:model")` и `ProviderConfig.ResolveKeys()` (пул ключей)

### Milestone 1 — один провайдер, один тул
- [ ] `openai.go` (стрим, текст)
- [ ] `mock.go` + тесты роутера
- [ ] `read_file` + `shell`
- [ ] headless CLI: `harness "прочитай main.go"`

### Milestone 2 — tool calls
- [ ] парсинг tool_calls в стриме
- [ ] реестр тулов + валидация JSON Schema
- [ ] агентный цикл с лимитом шагов
- [ ] `write_file`

### Milestone 3 — качество правок
- [ ] `hashline_edit` + хэши в `read_file`
- [ ] `grep`, `glob`
- [ ] усечение вывода shell

### Milestone 4 — Gemini + роутер
- [ ] `gemini.go`
- [ ] `router.go`: роли thinker/worker, ключи, фоллбэк
- [ ] тесты фоллбэка через `httptest`

### Milestone 5 — TUI
- [ ] Bubbletea-чат
- [ ] стриминг и индикаторы тулов
- [ ] отмена через контекст

### Milestone 6 — сессии и сабагенты
- [ ] сохранение/загрузка истории (`~/.harness/sessions/*.json`)
- [ ] сабагенты
- [ ] (опц.) MCP-интерфейс

---

## 14. Конвенции кода

- `internal/` для всего, что не является публичным API.
- Ошибки оборачивать: `fmt.Errorf("read_file: %w", err)`.
- Все внешние вызовы — только с `context.Context`.
- Никаких `panic` в рантайме агента; паника допустима только в `main` при фатальном конфиге.
- Логи — `slog`, уровень через флаг `-v`.
- Тесты: табличные, `httptest` для провайдеров, `t.TempDir()` для файловых тулов.
- Форматирование: `gofmt` / `go vet` обязательны.
- Комментарии — по делу; экспортируемые типы документировать.

---

## 15. Безопасность

- Ключи только из env.
- `shell` — таймаут обязателен, рабочий каталог явный.
- Не выполнять команды, пришедшие «из воздуха»: только через tool call модели.
- (После MVP) подтверждение деструктивных операций в TUI. В MVP — без подтверждений.
- Не логировать ключи и полные заголовки авторизации.

---

## 16. Принятые решения (зафиксировано)

1. **Имя проекта/бинарника** — `harness`.
2. **Конфиг** — JSON (`encoding/json`), без сторонних библиотек.
3. **Codex** — подписка ChatGPT через **прокси**, отдельный адаптер `codex_proxy` (см. 7.4).
4. **Grep** — использовать `ripgrep`, если он есть в PATH; иначе fallback на Go.
5. **Хранение сессий** — JSON-файлы в `~/.harness/sessions/*.json`.
6. **Подтверждение shell** — в MVP **не спрашивать**; добавить после MVP.

---

## 17. Как начать (команды)

```powershell
cd C:\Users\slywater\projects\Harness
go mod init github.com/slywater/harness
# зависимости (по мере надобности):
go get github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss github.com/charmbracelet/bubbles
go build ./...
go vet ./...
go test ./...
```

Внешние утилиты (опционально):
- `rg` (ripgrep) — ускоряет `grep`, но без него есть fallback.

Проверка окружения:
- Go: `go version` (должен быть ≥ 1.26)
- ОС: Windows (PowerShell), но код не должен быть завязан на Windows.

---

## 18. Ресурсы для вдохновения (читать, но не копировать вслепую)

- OpenCode — стабильное ядро кодинг-агента, TS.
- OMP / Pi — hashline edit, ферма аккаунтов, сабагенты.
- Aider — точечное редактирование через diff/SEARCH-REPLACE.
- DeepSeek Harness — идея «всё есть плагин», но там же и грабли (не повторять).
- OpenAI SSE и Gemini streaming API — официальная документация.

---

## 19. Критерии готовности MVP

- [ ] `harness` запускается одним бинарником.
- [ ] Читает конфиг, подхватывает ключи из env.
- [ ] Ведёт диалог с ChatGPT и Gemini (хотя бы текст).
- [ ] Выполняет `read_file` / `hashline_edit` / `shell` по просьбе модели.
- [ ] Правит существующий файл, не ломая остальное.
- [ ] Переживает 429/502 с фоллбэком.
- [ ] TUI показывает стрим и статус тулов.
- [ ] Есть тесты на роутер, hashline и парсер стрима.

---

*Конец документа. При сомнениях — сначала обнови этот файл, потом код.*
