package engine

// EventKind — тип события движка для UI.
type EventKind int

const (
	EvToken EventKind = iota
	EvThinking
	EvToolStart
	EvToolEnd
	EvDone
	EvError
)

// Event — событие, которое движок отправляет интерфейсу.
type Event struct {
	Kind     EventKind
	Text     string
	ToolName string
	ToolArgs string
	ToolOut  string
	Err      error
}
