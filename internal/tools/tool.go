package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Result — результат выполнения инструмента.
type Result struct {
	Output  string
	IsError bool
}

// Tool — интерфейс инструмента, доступного модели.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON Schema параметров
	// Run получает сырой JSON аргументов и возвращает результат.
	// Ошибки выполнения возвращаются как Result{IsError: true}, а не паникой.
	Run(ctx context.Context, args json.RawMessage) (Result, error)
}

// Registry — реестр инструментов по имени.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry создаёт реестр и регистрирует переданные инструменты.
func NewRegistry(list ...Tool) *Registry {
	r := &Registry{tools: make(map[string]Tool, len(list))}
	for _, t := range list {
		r.Register(t)
	}
	return r
}

// Register добавляет инструмент. Одноимённый инструмент заменяется.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get возвращает инструмент по имени.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List возвращает инструменты в стабильном (алфавитном) порядке.
func (r *Registry) List() []Tool {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]Tool, 0, len(names))
	for _, n := range names {
		out = append(out, r.tools[n])
	}
	return out
}

// Run выполняет инструмент по имени. Неизвестный инструмент — ошибка в Result.
func (r *Registry) Run(ctx context.Context, name string, args json.RawMessage) (Result, error) {
	t, ok := r.tools[name]
	if !ok {
		return Result{Output: fmt.Sprintf("unknown tool %q", name), IsError: true}, nil
	}
	return t.Run(ctx, args)
}
