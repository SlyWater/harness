package llm

import (
	"context"
	"fmt"
)

// MockProvider — провайдер для тестов и отладки без сети.
type MockProvider struct {
	NameStr  string
	StreamFn func(ctx context.Context, req Request) (<-chan StreamEvent, error)
}

func (m *MockProvider) Name() string {
	if m.NameStr != "" {
		return m.NameStr
	}
	return "mock"
}

func (m *MockProvider) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	if m.StreamFn != nil {
		return m.StreamFn(ctx, req)
	}
	return nil, fmt.Errorf("mock: StreamFn is nil")
}

// EventsProvider возвращает mock-провайдера, который отдаёт заданные события.
func EventsProvider(events ...StreamEvent) *MockProvider {
	return &MockProvider{
		StreamFn: func(ctx context.Context, req Request) (<-chan StreamEvent, error) {
			ch := make(chan StreamEvent)
			go func() {
				defer close(ch)
				for _, ev := range events {
					select {
					case <-ctx.Done():
						return
					case ch <- ev:
					}
				}
			}()
			return ch, nil
		},
	}
}
