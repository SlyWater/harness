package llm

import (
	"context"
	"encoding/json"
)

// Role — роль сообщения в диалоге.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message — одно сообщение истории.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall — запрос модели на вызов инструмента.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolSchema — описание инструмента для модели (JSON Schema).
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Request — запрос к провайдеру.
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Tools       []ToolSchema
	Temperature float64
	MaxTokens   int
}

// EventType — тип события стрима.
type EventType int

const (
	EventText EventType = iota
	EventThinking
	EventToolCall
	EventDone
	EventError
)

// Usage — расход токенов (если провайдер отдаёт).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// StreamEvent — единый формат события для всех провайдеров.
type StreamEvent struct {
	Type     EventType
	Text     string
	ToolCall *ToolCall
	Usage    *Usage
	Err      error
}

// Provider — интерфейс любого LLM-провайдера.
type Provider interface {
	Name() string
	// Stream отправляет события в канал и закрывает его по завершении.
	// Отмена через ctx должна немедленно прекращать чтение HTTP-потока.
	Stream(ctx context.Context, req Request) (<-chan StreamEvent, error)
}
