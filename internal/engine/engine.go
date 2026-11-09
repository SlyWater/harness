package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/slywater/harness/internal/llm"
	"github.com/slywater/harness/internal/tools"
)

// Options — настройки движка.
type Options struct {
	Provider    llm.Provider
	Model       string
	System      string
	Tools       *tools.Registry
	MaxSteps    int
	MaxTokens   int
	Temperature float64
}

// Engine — агентный ReAct-цикл.
type Engine struct {
	opts Options
}

// New создаёт движок. MaxSteps по умолчанию 25.
func New(opts Options) *Engine {
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 25
	}
	if opts.Tools == nil {
		opts.Tools = tools.NewRegistry()
	}
	return &Engine{opts: opts}
}

func (e *Engine) schemas() []llm.ToolSchema {
	list := e.opts.Tools.List()
	out := make([]llm.ToolSchema, 0, len(list))
	for _, t := range list {
		out = append(out, llm.ToolSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Schema(),
		})
	}
	return out
}

// Run выполняет задачу до финального ответа или лимита шагов.
// emit может быть nil.
func (e *Engine) Run(ctx context.Context, input string, emit func(Event)) (string, error) {
	if emit == nil {
		emit = func(Event) {}
	}

	history := []llm.Message{{Role: llm.RoleUser, Content: input}}

	for step := 0; step < e.opts.MaxSteps; step++ {
		req := llm.Request{
			Model:       e.opts.Model,
			System:      e.opts.System,
			Messages:    history,
			Tools:       e.schemas(),
			Temperature: e.opts.Temperature,
			MaxTokens:   e.opts.MaxTokens,
		}

		stream, err := e.opts.Provider.Stream(ctx, req)
		if err != nil {
			emit(Event{Kind: EvError, Err: err})
			return "", err
		}

		var content strings.Builder
		var calls []llm.ToolCall

		for ev := range stream {
			switch ev.Type {
			case llm.EventText:
				content.WriteString(ev.Text)
				emit(Event{Kind: EvToken, Text: ev.Text})
			case llm.EventThinking:
				emit(Event{Kind: EvThinking, Text: ev.Text})
			case llm.EventToolCall:
				if ev.ToolCall != nil {
					calls = append(calls, *ev.ToolCall)
				}
			case llm.EventError:
				emit(Event{Kind: EvError, Err: ev.Err})
				return "", ev.Err
			}
		}

		assistant := llm.Message{
			Role:      llm.RoleAssistant,
			Content:   content.String(),
			ToolCalls: calls,
		}
		history = append(history, assistant)

		if len(calls) == 0 {
			emit(Event{Kind: EvDone, Text: assistant.Content})
			return assistant.Content, nil
		}

		for _, tc := range calls {
			emit(Event{Kind: EvToolStart, ToolName: tc.Name, ToolArgs: string(tc.Arguments)})

			res, err := e.opts.Tools.Run(ctx, tc.Name, tc.Arguments)
			if err != nil {
				res = tools.Result{Output: "tool error: " + err.Error(), IsError: true}
			}

			out := res.Output
			if res.IsError {
				out = "ERROR: " + out
			}
			emit(Event{Kind: EvToolEnd, ToolName: tc.Name, ToolOut: out})
			history = append(history, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: tc.ID,
				Content:    out,
			})
		}
	}

	err := fmt.Errorf("engine: exceeded max steps (%d)", e.opts.MaxSteps)
	emit(Event{Kind: EvError, Err: err})
	return "", err
}
