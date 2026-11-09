package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider — провайдер, совместимый с OpenAI Chat Completions API.
// Используется для OpenAI, DeepSeek, OpenRouter, Together, vLLM и codex-прокси.
type OpenAIProvider struct {
	name    string
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewOpenAI создаёт OpenAI-совместимый провайдер.
func NewOpenAI(name, baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 60 * time.Second,
			},
		},
	}
}

func (p *OpenAIProvider) Name() string { return p.name }

// HTTPError — ошибка HTTP-уровня (нужна роутеру для фоллбэка).
type HTTPError struct {
	Provider   string
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: http %s", e.Provider, e.Status)
	}
	return fmt.Sprintf("%s: http %s: %s", e.Provider, e.Status, e.Body)
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream"`
}

type openAIMessage struct {
	Role       Role                    `json:"role"`
	Content    string                  `json:"content,omitempty"`
	ToolCalls  []openAIMessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string                  `json:"tool_call_id,omitempty"`
}

type openAIMessageToolCall struct {
	ID       string                `json:"id"`
	Type     string                `json:"type"`
	Function openAIMessageFunction `json:"function"`
}

type openAIMessageFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (p *OpenAIProvider) buildRequest(req Request) openAIRequest {
	msgs := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, openAIMessage{Role: RoleSystem, Content: req.System})
	}
	for _, m := range req.Messages {
		om := openAIMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, openAIMessageToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: openAIMessageFunction{
					Name:      tc.Name,
					Arguments: string(tc.Arguments),
				},
			})
		}
		msgs = append(msgs, om)
	}

	out := openAIRequest{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

// Stream отправляет запрос и читает SSE-поток.
func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	payload, err := json.Marshal(p.buildRequest(req))
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("openai: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, &HTTPError{
			Provider:   p.name,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	ch := make(chan StreamEvent)
	go p.readStream(ctx, resp.Body, ch)
	return ch, nil
}

type chunkFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chunkToolCall struct {
	Index    int           `json:"index"`
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function chunkFunction `json:"function"`
}

type chunkDelta struct {
	Content          string          `json:"content"`
	ReasoningContent string          `json:"reasoning_content"`
	Reasoning        string          `json:"reasoning"`
	ToolCalls        []chunkToolCall `json:"tool_calls"`
}

type chunkChoice struct {
	Delta        chunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason"`
}

type chunkUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type chunkError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type chatChunk struct {
	Choices []chunkChoice `json:"choices"`
	Usage   *chunkUsage   `json:"usage"`
	Error   *chunkError   `json:"error"`
}

type toolAcc struct {
	id   string
	name string
	args strings.Builder
}

func (p *OpenAIProvider) readStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamEvent) {
	defer close(ch)
	defer body.Close()

	send := func(ev StreamEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case ch <- ev:
			return true
		}
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	accs := make(map[int]*toolAcc)
	var order []int
	var usage *Usage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk chatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			if !send(StreamEvent{Type: EventError, Err: fmt.Errorf("openai: decode chunk: %w", err)}) {
				return
			}
			continue
		}
		if chunk.Error != nil {
			send(StreamEvent{Type: EventError, Err: fmt.Errorf("openai: %s: %s", chunk.Error.Type, chunk.Error.Message)})
			return
		}
		if chunk.Usage != nil {
			usage = &Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
			}
		}

		for _, choice := range chunk.Choices {
			d := choice.Delta
			if d.ReasoningContent != "" {
				if !send(StreamEvent{Type: EventThinking, Text: d.ReasoningContent}) {
					return
				}
			}
			if d.Reasoning != "" {
				if !send(StreamEvent{Type: EventThinking, Text: d.Reasoning}) {
					return
				}
			}
			if d.Content != "" {
				if !send(StreamEvent{Type: EventText, Text: d.Content}) {
					return
				}
			}
			for _, tc := range d.ToolCalls {
				acc := accs[tc.Index]
				if acc == nil {
					acc = &toolAcc{}
					accs[tc.Index] = acc
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				acc.args.WriteString(tc.Function.Arguments)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		send(StreamEvent{Type: EventError, Err: fmt.Errorf("openai: read stream: %w", err)})
		return
	}

	for _, idx := range order {
		acc := accs[idx]
		args := acc.args.String()
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		if !json.Valid([]byte(args)) {
			send(StreamEvent{Type: EventError, Err: fmt.Errorf("openai: invalid arguments for tool %q: %s", acc.name, args)})
			return
		}
		if !send(StreamEvent{Type: EventToolCall, ToolCall: &ToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: json.RawMessage(args),
		}}) {
			return
		}
	}

	send(StreamEvent{Type: EventDone, Usage: usage})
}
