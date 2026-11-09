package llm

import (
	"fmt"

	"github.com/slywater/harness/internal/config"
)

// NewProvider создаёт провайдера по настройкам из конфига.
func NewProvider(pc config.ProviderConfig) (Provider, error) {
	switch pc.Type {
	case "openai_compatible", "codex_proxy":
		key, err := pc.APIKey()
		if err != nil {
			return nil, err
		}
		return NewOpenAI(pc.Type, pc.BaseURL, key), nil
	case "gemini":
		return nil, fmt.Errorf("llm: provider type %q is not implemented yet", pc.Type)
	default:
		return nil, fmt.Errorf("llm: unknown provider type %q", pc.Type)
	}
}
